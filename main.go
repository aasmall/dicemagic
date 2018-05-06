package main

import (
	"net/http"

	"github.com/aasmall/dicemagic/api"
	"github.com/aasmall/dicemagic/www"

	"google.golang.org/appengine"
)

func main() {
	http.HandleFunc("/", www.Handle)
	http.HandleFunc("/api/slack/roll/", api.SlackRollHandler)
	http.HandleFunc("/api/dflow/", api.DialogueWebhookHandler)
	appengine.Main()
}
