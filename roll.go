package main

import (
	"context"
	"net/http"

	"google.golang.org/appengine"
)

//CommandType is an enum of supported commands
type CommandType int

const (
	//DiceRoll represents a command to roll dice.
	DiceRoll CommandType = iota
	//Decision represents a command to make a decision
	Decision
)

//go:generate stringer -type=CommandType

type Command interface {
	Save(ctx context.Context, namespace string, key string) error
	Get(ctx context.Context, namespace string, key string) error
	FromString(inputString ...string) error
	String() string
}

func main() {
	http.HandleFunc("/", rootHandle)
	http.HandleFunc("/slack/roll/", slackRoll)
	http.HandleFunc("/dflow/", dialogueWebhookHandler)
	appengine.Main()
}
