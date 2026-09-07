package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"uuid"

	"github.com/coder/websocket"
)

type Category = int

const (
	Opti Category = iota
	Real
	Pess
)

// TODO: pull from env
const password = "test"

var invalidPasswordJson = []byte(`{"action":"invalidPassword"}`)
var unknownIdJson = []byte(`{"action":"unknownId"}`)
var nameDuplicateJson = []byte(`{"action":"nameDuplicate"}`)
var serverError = []byte(`{"action":"serverError"}`)
var invalidPayloadJson = []byte(`{"action":"invalidPayload"}`)

type User struct {
	id           string
	broadcast    chan []byte
	ctx          context.Context
	conn         *websocket.Conn
	Name         string    `json:"name"`
	Connected    bool      `json:"connected"`
	NotAnswering bool      `json:"notAnswering"`
	Ping         uint16    `json:"ping"`
	Bets         [3]string `json:"bets"`
}

func newUser(ctx context.Context, conn *websocket.Conn, name string) *User {
	return &User{
		id:           uuid.New().String(),
		broadcast:    make(chan []byte, 10),
		ctx:          ctx,
		conn:         conn,
		Name:         name,
		Connected:    true,
		NotAnswering: false,
		Ping:         999,
		Bets:         [3]string{"", "", ""},
	}
}

func (u *User) listen() {
	for {
		select {
		case <-u.ctx.Done():
			return

		case msg, ok := <-u.broadcast:
			if !ok {
				return
			}

			ctx, cancel := context.WithTimeout(u.ctx, 5*time.Second)
			err := u.conn.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				log.Println("write error:", err)
				return
			}
		}
	}
}

func (u *User) send(msg []byte) {
	log.Printf("sending msg to user '%s' (%s): %s", u.Name, u.id, msg)
	u.broadcast <- msg
}

func writeToSocket(ctx context.Context, conn *websocket.Conn, msg []byte) {
	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		log.Printf("write failed: %v", err)
		return
	}
}

func writeToUsers(users []*User, msg []byte) {
	for _, user := range users {
		if user.Connected {
			user.send(msg)
		}
	}
}

type State struct {
	mu    sync.Mutex
	users []*User
}

func (s *State) getUserById(id string) *User {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range s.users {
		if user.id == id {
			return user
		}
	}

	return nil
}

func (s *State) getUserByConn(conn *websocket.Conn) *User {
	for _, user := range s.users {
		if user.conn == conn {
			return user
		}
	}
	return nil
}

func (s *State) getSnapshotForInit() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	users := make([]User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, *user)
	}
	return json.Marshal(InitAnswer{Action: "init", Data: users})
}

func (s *State) hasName(name string) bool {
	for _, user := range s.users {
		if user.Name == name {
			return true
		}
	}
	return false
}

func newState() State {
	return State{
		users: []*User{},
	}
}

type Payload struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type LoginPayload struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type InitPayload struct {
	Id string `json:"id"`
}

type VotePayload struct {
	Id       string   `json:"id"`
	Value    string   `json:"value"`
	Category Category `json:"category"`
}

type LoginAnswer struct {
	Action string `json:"action"`
	Uuid   string `json:"uuid"`
}

type NewUserAnswer struct {
	Action string `json:"action"`
	Name   string `json:"name"`
}

type InitAnswer struct {
	Action string `json:"action"`
	Data   []User `json:"data"`
}

type VoteData struct {
	Name     string   `json:"name"`
	Value    string   `json:"value"`
	Category Category `json:"category"`
}

type VoteAnswer struct {
	Action string   `json:"action"`
	Data   VoteData `json:"data"`
}

type DisconnectAnswer struct {
	Action string `json:"action"`
	Name   string `json:"name"`
}

func handleLogin(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	var data LoginPayload

	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("unmarshal failed: %v", err)
		writeToSocket(ctx, conn, invalidPayloadJson)
		return
	}

	if data.Password != password {
		writeToSocket(ctx, conn, invalidPasswordJson)
		return
	}

	if state.hasName(data.Name) {
		writeToSocket(ctx, conn, nameDuplicateJson)
		return
	}

	user := newUser(ctx, conn, data.Name)
	log.Printf("user '%s' (%s) listening to messages", user.Name, user.id)
	go user.listen()

	state.mu.Lock()
	state.users = append(state.users, user)
	state.mu.Unlock()

	answer, err := json.Marshal(LoginAnswer{Action: "login", Uuid: user.id})
	if err != nil {
		log.Printf("marshal failed: %v", err)
		writeToSocket(ctx, conn, serverError)
		return
	}

	user.send(answer)

	answer, err = json.Marshal(NewUserAnswer{Action: "newUser", Name: user.Name})
	if err != nil {
		log.Printf("marshal failed: %v", err)
		writeToSocket(ctx, conn, serverError)
		return
	}

	writeToUsers(state.users, answer)
}

func handleInit(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	var data InitPayload

	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("unmarshal failed: %v", err)
		writeToSocket(ctx, conn, invalidPayloadJson)
		return
	}

	if user := state.getUserById(data.Id); user != nil {
		answer, err := state.getSnapshotForInit()
		if err != nil {
			log.Printf("marshal failed: %v", err)
			writeToSocket(ctx, conn, serverError)
			return
		}
		user.send(answer)
		return
	}

	writeToSocket(ctx, conn, unknownIdJson)
}

func handleVote(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	var data VotePayload

	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("unmarshal failed: %v", err)
		writeToSocket(ctx, conn, invalidPayloadJson)
		return
	}

	if user := state.getUserById(data.Id); user != nil {
		answer, err := json.Marshal(VoteAnswer{Action: "vote", Data: VoteData{
			Value:    data.Value,
			Category: data.Category,
			Name:     user.Name,
		}})
		if err != nil {
			log.Printf("marshal failed: %v", err)
			writeToSocket(ctx, conn, serverError)
			return
		}
		writeToUsers(state.users, answer)
		return
	}

	writeToSocket(ctx, conn, unknownIdJson)
}

func handler(state *State) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"localhost:5173"},
		})
		if err != nil {
			log.Printf("accept failed: %v", err)
			return
		}
		defer conn.CloseNow()

		ctx := r.Context()

		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				log.Printf("read failed: %v", err)

				state.mu.Lock()

				user := state.getUserByConn(conn)
				if user == nil {
					return
				}

				user.Connected = false

				state.mu.Unlock()

				answer, err := json.Marshal(DisconnectAnswer{Action: "disconnect", Name: user.Name})
				if err != nil {
					log.Printf("marshal failed: %v", err)
				}

				writeToUsers(state.users, answer)
				return
			}

			var payload Payload
			if err := json.Unmarshal(msg, &payload); err != nil {
				log.Printf("unmarshal failed: %v", err)
				writeToSocket(ctx, conn, serverError)
				return
			}

			switch payload.Action {
			case "login":
				handleLogin(state, ctx, conn, &payload)
			case "init":
				handleInit(state, ctx, conn, &payload)
			case "vote":
				handleVote(state, ctx, conn, &payload)
			}
		}
	}
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer stop()

	state := newState()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handler(&state)),
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down, draining connections...")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)
	defer cancel()

	srv.Shutdown(shutdownCtx)
}
