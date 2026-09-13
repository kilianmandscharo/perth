package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coder/websocket"
)

type Category = int

const (
	Opti Category = iota
	Real
	Pess
)

const (
	numberOfBroadcastRoutines = 3
	pingIntervalInSeconds     = 5
	messageTimeoutInSeconds   = 5
	userTimeoutInSeconds      = 15
)

const password = "test"

// TODO:
// - pull pw from env
// - pull origin from env
// - when do disconnected users get removed?

func handleLogin(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	data, err := parseLoginData(payload.Data)
	if err != nil {
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	if data.Password != password {
		state.send(conn, ctx, invalidPasswordJson)
		return
	}

	state.mu.Lock()

	if user := state.getUserByName(data.Name); user != nil {
		if !user.Connected {
			state.removeUserByName(data.Name)
		} else {
			state.mu.Unlock()
			state.send(conn, ctx, nameDuplicateJson)
			return
		}
	}

	user := newUser(conn, ctx, data.Name)

	loginAnswer, err := loginAnswerJson(user.id)
	if err != nil {
		state.mu.Unlock()
		state.send(conn, ctx, serverErrorJson)
		return
	}

	newUserNotification, err := newUserNotificationJson(user.Name)
	if err != nil {
		state.mu.Unlock()
		state.send(conn, ctx, serverErrorJson)
		return
	}

	state.addUser(user)
	userMessage := state.getMessageTo(user, loginAnswer)
	otherUsersMessages := state.getMessagesExceptFor(user, newUserNotification)

	state.mu.Unlock()

	state.sendMessage(userMessage)
	state.sendMessages(otherUsersMessages)
}

func handleInit(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	data, err := parseIdData(payload.Data)
	if err != nil {
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	state.mu.Lock()

	user := state.getUserById(data.Id)
	if user == nil {
		state.mu.Unlock()
		state.send(conn, ctx, unknownIdJson)
		return
	}

	wasConnected := user.Connected
	user.reconnect(conn, ctx)

	initAnswer, err := initAnswerJson(state.getUsers())
	if err != nil {
		state.mu.Unlock()
		state.send(conn, ctx, serverErrorJson)
		return
	}

	initMessage := state.getMessageTo(user, initAnswer)

	if wasConnected {
		state.mu.Unlock()
		state.sendMessage(initMessage)
		return
	}

	reconnectNotification, err := reconnectNotificationJson(user.Name)
	if err != nil {
		state.mu.Unlock()
		state.send(conn, ctx, serverErrorJson)
		return
	}

	reconnectMessages := state.getMessagesExceptFor(user, reconnectNotification)

	state.mu.Unlock()

	state.sendMessage(initMessage)
	state.sendMessages(reconnectMessages)
}

func handleVote(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	data, err := parseVoteData(payload.Data)
	if err != nil {
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	state.mu.Lock()

	user := state.getUserById(data.Id)
	if user == nil {
		state.mu.Unlock()
		state.send(conn, ctx, unknownIdJson)
		return
	}

	notification, err := voteNotificationJson(user.Name, data.Value, data.Category)
	if err != nil {
		state.mu.Unlock()
		state.send(conn, ctx, serverErrorJson)
		return
	}

	messages := state.getMessages(notification)
	state.mu.Unlock()
	state.sendMessages(messages)
}

func handleReset(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	data, err := parseIdData(payload.Data)
	if err != nil {
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	state.mu.Lock()

	user := state.getUserById(data.Id)
	if user == nil {
		state.mu.Unlock()
		state.send(conn, ctx, unknownIdJson)
		return
	}

	state.resetVotes()
	messages := state.getMessages(resetJson)
	state.mu.Unlock()
	state.sendMessages(messages)
}

func handlePong(state *State, ctx context.Context, conn *websocket.Conn, payload *Payload) {
	data, err := parseIdData(payload.Data)
	if err != nil {
		state.send(conn, ctx, invalidPayloadJson)
		return
	}

	state.mu.Lock()

	user := state.getUserById(data.Id)
	if user == nil {
		state.mu.Unlock()
		state.send(conn, ctx, unknownIdJson)
		return
	}

	if user.lastPing.IsZero() {
		state.mu.Unlock()
		log.Println("lastPing has zero value for user", user.Name)
		return
	}

	now := time.Now()
	user.Ping = now.Sub(user.lastPing).Milliseconds()
	user.answered = true

	notification, err := pingUpdateNotificationJson(user.Name, user.Ping)
	if err != nil {
		state.mu.Unlock()
		state.send(conn, ctx, serverErrorJson)
		return
	}

	messages := state.getMessages(notification)
	state.mu.Unlock()
	state.sendMessages(messages)
}

func handler(state *State) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"localhost:5173"},
		})
		if err != nil {
			log.Println("accept failed:", err)
			return
		}
		defer conn.CloseNow()

		log.Println("new connection established")

		ctx := r.Context()

		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				log.Println("read failed:", err)

				state.mu.Lock()

				user := state.getUserByConn(conn)
				if user == nil {
					log.Println("conn not connected to any user")
					state.mu.Unlock()
					return
				}

				user.disconnect()

				notification, innerErr := disconnectNotificationJson(user.Name)
				if innerErr != nil {
					state.mu.Unlock()
					return
				}

				messages := state.getMessages(notification)
				state.mu.Unlock()
				state.sendMessages(messages)
				return
			}

			log.Println("received message:", string(msg))

			payload, err := parsePayload(msg)
			if err != nil {
				state.send(conn, ctx, invalidPayloadJson)
				continue
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
			default:
				state.send(conn, ctx, invalidPayloadJson)
			}
		}
	}
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer stop()

	ticker := time.NewTicker(pingIntervalInSeconds * time.Second)
	broadcast := make(chan Message)
	state := newState(broadcast)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(handler(&state)),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case t := <-ticker.C:
				state.mu.Lock()

				state.setLastPingForAll(t)
				timedOutUsers := state.checkUserTimeout(t)
				pingMessages := state.getMessages(pingJson)

				var disconnectMessages []Message
				for _, name := range timedOutUsers {
					notification, err := disconnectNotificationJson(name)
					if err != nil {
						continue
					}
					disconnectMessages = append(disconnectMessages, state.getMessages(notification)...)
				}

				state.mu.Unlock()

				state.sendMessages(disconnectMessages)
				state.sendMessages(pingMessages)
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
						continue
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
