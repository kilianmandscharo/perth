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

type StateItem struct {
	id           string
	conn         *websocket.Conn
	Name         string    `json:"name"`
	Connected    bool      `json:"connected"`
	NotAnswering bool      `json:"notAnswering"`
	Ping         uint16    `json:"ping"`
	Bets         [3]string `json:"bets"`
}

type State struct {
	mu    sync.Mutex
	items []StateItem
}

func (s *State) hasId(id string) bool {
	for _, user := range s.items {
		if user.id == id {
			return true
		}
	}
	return false
}

func (s *State) getConnById(id string) *websocket.Conn {
	for _, user := range s.items {
		if user.id == id {
			return user.conn
		}
	}
	panic("user not found")
}

func (s *State) getNameById(id string) string {
	for _, user := range s.items {
		if user.id == id {
			return user.Name
		}
	}
	panic("user not found")
}

func newState() State {
	return State{
		items: []StateItem{},
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
	Action string      `json:"action"`
	Data   []StateItem `json:"data"`
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

type UnknownIdAnswer struct {
	Action string `json:"action"`
}

// TODO: pull from env
const password = "test"

func addUser(state *State, name, id string, conn *websocket.Conn) {
	state.mu.Lock()
	defer state.mu.Unlock()

	// TODO: what if name already exists? -> send error

	state.items = append(state.items, StateItem{
		Name:      name,
		Connected: true,
		conn:      conn,
		id:        id,
	})
}

func broadcastNewUser(r *http.Request, state *State, name, id string) {
	loginAnswer, err := json.Marshal(LoginAnswer{Action: "login", Uuid: id})
	if err != nil {
		log.Printf("marshal failed: %v", err)
		return
	}

	ctx := r.Context()
	conn := state.getConnById(id)

	if err := conn.Write(ctx, websocket.MessageText, loginAnswer); err != nil {
		log.Printf("write failed: %v", err)
		return
	}

	newUserAnswer, err := json.Marshal(NewUserAnswer{Action: "newUser", Name: name})
	if err != nil {
		log.Printf("marshal failed: %v", err)
		return
	}

	for _, user := range state.items {
		if err := user.conn.Write(ctx, websocket.MessageText, newUserAnswer); err != nil {
			log.Printf("write failed: %v", err)
			return
		}
	}
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
				return
			}

			var payload Payload
			if err := json.Unmarshal(msg, &payload); err != nil {
				log.Printf("unmarshal failed: %v", err)
				return
			}

			log.Printf("action: %s", payload.Action)

			switch payload.Action {
			case "login":
				var data LoginPayload
				if err := json.Unmarshal(payload.Data, &data); err != nil {
					log.Printf("unmarshal failed: %v", err)
					return
				}

				if data.Password != password {
					if err := conn.Write(ctx, websocket.MessageText, []byte("login failed")); err != nil {
						return
					}
				} else {
					id := uuid.New().String()
					addUser(state, data.Name, id, conn)
					broadcastNewUser(r, state, data.Name, id)
				}
			case "init":
				var data InitPayload
				if err := json.Unmarshal(payload.Data, &data); err != nil {
					log.Printf("unmarshal failed: %v", err)
					return
				}

				if !state.hasId(data.Id) {
					unknownIdAnswer, err := json.Marshal(UnknownIdAnswer{Action: "unknownId"})
					if err != nil {
						log.Printf("marshal failed: %v", err)
						return
					}

					if err := conn.Write(ctx, websocket.MessageText, unknownIdAnswer); err != nil {
						return
					}
				} else {
					initAnswer, err := json.Marshal(InitAnswer{Action: "init", Data: state.items})
					if err != nil {
						log.Printf("marshal failed: %v", err)
						return
					}

					if err := conn.Write(ctx, websocket.MessageText, initAnswer); err != nil {
						return
					}
				}
			case "vote":
				var data VotePayload
				if err := json.Unmarshal(payload.Data, &data); err != nil {
					log.Printf("unmarshal failed: %v", err)
					return
				}

				if !state.hasId(data.Id) {
					unknownIdAnswer, err := json.Marshal(UnknownIdAnswer{Action: "unknownId"})
					if err != nil {
						log.Printf("marshal failed: %v", err)
						return
					}

					if err := conn.Write(ctx, websocket.MessageText, unknownIdAnswer); err != nil {
						return
					}
				} else {
					voteAnswer, err := json.Marshal(VoteAnswer{Action: "vote", Data: VoteData{
						Value:    data.Value,
						Category: data.Category,
						Name:     state.getNameById(data.Id),
					}})
					if err != nil {
						log.Printf("marshal failed: %v", err)
						return
					}

					for _, user := range state.items {
						if err := user.conn.Write(ctx, websocket.MessageText, voteAnswer); err != nil {
							log.Printf("write failed: %v", err)
							return
						}
					}
				}
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
