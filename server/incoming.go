package main

import (
	"encoding/json"
	"log"
)

func unmarshalJson[T any](data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	if err != nil {
		log.Println("error unmarshaling", data, ":", err)
		return v, err
	}
	return v, nil
}

type Payload struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

func parsePayload(data []byte) (Payload, error) {
	return unmarshalJson[Payload](data)
}

type IdData struct {
	Id string `json:"id"`
}

func parseIdData(data []byte) (IdData, error) {
	return unmarshalJson[IdData](data)
}

type LoginData struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func parseLoginData(data []byte) (LoginData, error) {
	return unmarshalJson[LoginData](data)
}

type VoteData struct {
	Id       string   `json:"id"`
	Value    string   `json:"value"`
	Category Category `json:"category"`
}

func parseVoteData(data []byte) (VoteData, error) {
	return unmarshalJson[VoteData](data)
}
