package main

import (
	"context"
	"sync"
	"time"
	"uuid"

	"github.com/coder/websocket"
)

type Message struct {
	msg  []byte
	conn *websocket.Conn
	ctx  context.Context
}

type User struct {
	id           string
	ctx          context.Context
	conn         *websocket.Conn
	lastPing     time.Time
	Name         string    `json:"name"`
	Connected    bool      `json:"connected"`
	NotAnswering bool      `json:"notAnswering"`
	Ping         int64     `json:"ping"`
	Votes        [3]string `json:"votes"`
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
		Votes:        [3]string{"", "", ""},
	}
}

func (u *User) send(broadcast chan Message, msg []byte) {
	broadcast <- Message{
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
	broadcast chan Message
}

func newState(broadcast chan Message) State {
	return State{
		broadcast: broadcast,
		users:     []*User{},
	}
}

func (s *State) addUser(user *User) {
	s.users = append(s.users, user)
}

func (s *State) getUserById(id string) *User {
	for _, user := range s.users {
		if user.id == id {
			return user
		}
	}
	return nil
}

func (s *State) getUserByName(name string) *User {
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

func (s *State) getUsers() []User {
	users := make([]User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, *user)
	}
	return users
}

func (s *State) resetVotes() {
	for _, user := range s.users {
		for i := range user.Votes {
			user.Votes[i] = ""
		}
	}
}

func (s *State) removeUserByName(name string) {
	users := make([]*User, 0)
	for _, user := range s.users {
		if user.Name != name {
			users = append(users, user)
		}
	}
}

func (s *State) setLastPingForAll(now time.Time) {
	for _, user := range s.users {
		if user.Connected {
			user.lastPing = now
		}
	}
}

func (s *State) getMessagesExceptFor(exception *User, msg []byte) []Message {
	messages := make([]Message, 0)
	for _, user := range s.users {
		if user.Connected && user != exception {
			messages = append(messages, Message{
				conn: user.conn,
				ctx:  user.ctx,
				msg:  msg,
			})
		}
	}
	return messages
}

func (s *State) getMessages(msg []byte) []Message {
	return s.getMessagesExceptFor(nil, msg)
}

func (s *State) getMessageTo(user *User, msg []byte) Message {
	return Message{
		conn: user.conn,
		ctx:  user.ctx,
		msg:  msg,
	}
}

func (s *State) send(conn *websocket.Conn, ctx context.Context, msg []byte) {
	s.broadcast <- Message{
		msg:  msg,
		conn: conn,
		ctx:  ctx,
	}
}

func (s *State) sendMessage(msg Message) {
	s.broadcast <- msg
}

func (s *State) sendMessages(messages []Message) {
	for _, msg := range messages {
		s.broadcast <- msg
	}
}
