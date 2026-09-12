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

// TODO:
// - pull pw from env
// - pull origin from env
// - check state mutex
// - implement notAnswering
// - when do disconnected users get removed?

const password = "test"

var invalidPasswordJson = []byte(`{"action":"invalidPassword"}`)
var unknownIdJson = []byte(`{"action":"unknownId"}`)
var nameDuplicateJson = []byte(`{"action":"nameDuplicate"}`)
var serverError = []byte(`{"action":"serverError"}`)
var invalidPayloadJson = []byte(`{"action":"invalidPayload"}`)
var pingJson = []byte(`{"action":"ping"}`)
var resetJson = []byte(`{"action":"reset"}`)

type User struct {
	id           string
	ctx          context.Context
	conn         *websocket.Conn
	lastPing     time.Time
	Name         string    `json:"name"`
	Connected    bool      `json:"connected"`
	NotAnswering bool      `json:"notAnswering"`
	Ping         int64     `json:"ping"`
	Bets         [3]string `json:"bets"`
}

func newUser(conn *websocket.Conn, ctx context.Context, name string) *User {
	return &User{
		id:           uuid.New().String(),
		ctx:          ctx,
		conn:         conn,
		lastPing:     time.Now(),
		Name:         name,
		Connected:    true,
		NotAnswering: false,
		Ping:         999,
		Bets:         [3]string{"", "", ""},
	}
}

func (u *User) send(broadcast chan Broadcast, msg []byte) {
	broadcast <- Broadcast{
		msg:  msg,
		ctx:  u.ctx,
		conn: u.conn,
	}
}

func (u *User) disconnect() {
	u.Connected = false
	u.conn = nil
	u.ctx = nil
}

func (u *User) reconnect(conn *websocket.Conn, ctx context.Context) {
	u.Connected = true
	u.conn = conn
	u.ctx = ctx
}

type State struct {
	mu        sync.Mutex
	users     []*User
	broadcast chan Broadcast
}

func (s *State) addUser(user *User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = append(s.users, user)
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

func (s *State) getUserByName(name string) *User {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range s.users {
		if user.Name == name {
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

func (s *State) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range s.users {
		for i := range user.Bets {
			user.Bets[i] = ""
		}
	}
}

func (s *State) removeByName(name string) {
	users := make([]*User, 0)
	for _, user := range s.users {
		if user.Name != name {
			users = append(users, user)
		}
	}
}

func (s *State) pingAll(now time.Time) {
	for _, user := range s.users {
		if user.Connected {
			user.lastPing = now
			user.send(s.broadcast, pingJson)
		}
	}
}

func (s *State) sendToAll(msg []byte) {
	for _, user := range s.users {
		if user.Connected {
			user.send(s.broadcast, msg)
		}
	}
}

func (s *State) send(conn *websocket.Conn, ctx context.Context, msg []byte) {
	s.broadcast <- Broadcast{
		msg:  msg,
		conn: conn,
		ctx:  ctx,
	}
}

func newState(broadcast chan Broadcast) State {
	return State{
		broadcast: broadcast,
		users:     []*User{},
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

type ResetPayload struct {
	Id string `json:"id"`
}

type PongPayload struct {
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

type ReconnectAnswer struct {
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

type PingUpdateAnswer struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	Ping   int64  `json:"ping"`
}

func handleLogin(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	var data LoginPayload

	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("unmarshal failed: %v", err)
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	if data.Password != password {
		state.send(conn, ctx, invalidPasswordJson)
		return
	}

	if user := state.getUserByName(data.Name); user != nil {
		if !user.Connected {
			state.removeByName(data.Name)
		} else {
			state.send(conn, ctx, nameDuplicateJson)
			return
		}
	}

	user := newUser(conn, ctx, data.Name)

	answer, err := json.Marshal(LoginAnswer{Action: "login", Uuid: user.id})
	if err != nil {
		log.Printf("marshal failed: %v", err)
		state.send(conn, ctx, serverError)
		return
	}

	user.send(state.broadcast, answer)

	answer, err = json.Marshal(NewUserAnswer{Action: "newUser", Name: user.Name})
	if err != nil {
		log.Printf("marshal failed: %v", err)
		state.send(conn, ctx, serverError)
		return
	}

	state.sendToAll(answer)
	state.addUser(user)
}

func handleInit(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	var data InitPayload

	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("unmarshal failed: %v", err)
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	if user := state.getUserById(data.Id); user != nil {
		if !user.Connected {
			answer, err := json.Marshal(ReconnectAnswer{Action: "reconnect", Name: user.Name})
			if err != nil {
				log.Printf("marshal failed: %v", err)
				state.send(conn, ctx, serverError)
				return
			}
			state.sendToAll(answer)
			user.reconnect(conn, ctx)
		}

		answer, err := state.getSnapshotForInit()
		if err != nil {
			log.Printf("marshal failed: %v", err)
			state.send(conn, ctx, serverError)
			return
		}
		user.send(state.broadcast, answer)
		return
	}

	state.send(conn, ctx, unknownIdJson)
}

func handleVote(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	var data VotePayload

	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("unmarshal failed: %v", err)
		state.send(conn, ctx, invalidPayloadJson)
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
			state.send(conn, ctx, serverError)
			return
		}
		state.sendToAll(answer)
		return
	}

	state.send(conn, ctx, unknownIdJson)
}

func handleReset(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	var data ResetPayload

	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("unmarshal failed: %v", err)
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	if user := state.getUserById(data.Id); user != nil {
		state.reset()
		state.sendToAll(resetJson)
		return
	}

	state.send(conn, ctx, unknownIdJson)
}

func handlePong(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	var data PongPayload

	if err := json.Unmarshal(payload.Data, &data); err != nil {
		log.Printf("unmarshal failed: %v", err)
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	if user := state.getUserById(data.Id); user != nil {
		now := time.Now().UnixMilli()
		diff := (now - user.lastPing.UnixMilli())
		user.Ping = diff
		answer, err := json.Marshal(PingUpdateAnswer{Action: "pingUpdate", Name: user.Name, Ping: diff})
		if err != nil {
			log.Printf("marshal failed: %v", err)
			state.send(conn, ctx, serverError)
			return
		}
		state.sendToAll(answer)
		return
	}

	state.send(conn, ctx, unknownIdJson)
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

		log.Println("new connection established")

		ctx := r.Context()

		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				log.Printf("read failed: %v", err)

				user := state.getUserByConn(conn)
				if user == nil {
					return
				}

				user.disconnect()

				answer, err := json.Marshal(DisconnectAnswer{Action: "disconnect", Name: user.Name})
				if err != nil {
					log.Printf("marshal failed: %v", err)
				}

				state.sendToAll(answer)
				return
			}

			log.Println("received message:", string(msg))

			var payload Payload
			if err := json.Unmarshal(msg, &payload); err != nil {
				log.Printf("unmarshal failed: %v", err)
				state.send(conn, ctx, serverError)
				return
			}

			switch payload.Action {
			case "login":
				handleLogin(state, ctx, conn, &payload)
			case "init":
				handleInit(state, ctx, conn, &payload)
			case "vote":
				handleVote(state, ctx, conn, &payload)
			case "reset":
				handleReset(state, ctx, conn, &payload)
			case "pong":
				handlePong(state, ctx, conn, &payload)
			}
		}
	}
}

const (
	numberOfBroadcastRoutines = 3
	pingIntervalInSeconds     = 5
	messageTimeoutInSeconds   = 5
)

type Broadcast struct {
	msg  []byte
	conn *websocket.Conn
	ctx  context.Context
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer stop()

	// ticker := time.NewTicker(pingIntervalInSeconds * time.Second)
	broadcast := make(chan Broadcast)
	state := newState(broadcast)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handler(&state)),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
				// case t := <-ticker.C:
				// 	state.pingAll(t)
			}
		}
	}()

	for i := 0; i < numberOfBroadcastRoutines; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case b, ok := <-broadcast:
					if !ok {
						return
					}
					ctx, cancel := context.WithTimeout(b.ctx, messageTimeoutInSeconds*time.Second)
					log.Println("sending message:", string(b.msg))
					err := b.conn.Write(ctx, websocket.MessageText, b.msg)
					cancel()
					if err != nil {
						log.Println("write error:", err)
						return
					}
				}
			}
		}()
	}

	log.Println("listening on port 8080...")

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
