package main

import (
	"encoding/json"
	"log"
)

type actionMsg struct {
	Action string `json:"action"`
}

func mustMarshal(action string) []byte {
	data, err := json.Marshal(actionMsg{Action: action})
	if err != nil {
		panic(err)
	}
	return data
}

const (
	ActionLogin           = "login"
	ActionInit            = "init"
	ActionNewUser         = "newUser"
	ActionDisconnect      = "disconnect"
	ActionReconnect       = "reconnect"
	ActionVote            = "vote"
	ActionPingUpdate      = "pingUpdate"
	ActionInvalidPassword = "invalidPassword"
	ActionNameDuplicate   = "nameDuplicate"
	ActionUnknownId       = "unknownId"
	ActionServerError     = "serverError"
	ActionInvalidPayload  = "invalidPayload"
	ActionPing            = "ping"
	ActionReset           = "reset"
)

var (
	invalidPasswordJson = mustMarshal(ActionInvalidPassword)
	nameDuplicateJson   = mustMarshal(ActionNameDuplicate)
	unknownIdJson       = mustMarshal(ActionUnknownId)
	serverErrorJson     = mustMarshal(ActionServerError)
	invalidPayloadJson  = mustMarshal(ActionInvalidPayload)
	pingJson            = mustMarshal(ActionPing)
	resetJson           = mustMarshal(ActionReset)
)

func marshalJson[T any](v T) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Println("error marshaling", v, ":", err)
		return nil, err
	}
	return data, nil
}

type NameNotification struct {
	Action string `json:"action"`
	Name   string `json:"name"`
}

type LoginAnswer struct {
	Action string `json:"action"`
	Uuid   string `json:"uuid"`
}

func loginAnswerJson(uuid string) ([]byte, error) {
	return marshalJson(LoginAnswer{Action: ActionLogin, Uuid: uuid})
}

type InitAnswer struct {
	Action string `json:"action"`
	Data   []User `json:"data"`
}

func initAnswerJson(users []User) ([]byte, error) {
	return marshalJson(InitAnswer{Action: ActionInit, Data: users})
}

func newUserNotificationJson(name string) ([]byte, error) {
	return marshalJson(NameNotification{Action: ActionNewUser, Name: name})
}

func disconnectNotificationJson(name string) ([]byte, error) {
	return marshalJson(NameNotification{Action: ActionDisconnect, Name: name})
}

func reconnectNotificationJson(name string) ([]byte, error) {
	return marshalJson(NameNotification{Action: ActionReconnect, Name: name})
}

type VoteNotification struct {
	Action   string   `json:"action"`
	Name     string   `json:"name"`
	Value    string   `json:"value"`
	Category Category `json:"category"`
}

func voteNotificationJson(name string, value string, category Category) ([]byte, error) {
	return marshalJson(VoteNotification{Action: ActionVote, Name: name, Value: value, Category: category})
}

type PingUpdateNotification struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	Ping   int64  `json:"ping"`
}

func pingUpdateNotificationJson(name string, ping int64) ([]byte, error) {
	return marshalJson(PingUpdateNotification{Action: ActionPingUpdate, Name: name, Ping: ping})
}
