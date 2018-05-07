package main

import (
	"net/http"

	"github.com/aasmall/dicemagic/api"
	"github.com/aasmall/dicemagic/queue"
	"github.com/aasmall/dicemagic/www"
	"go.opencensus.io/trace"

	"google.golang.org/appengine"
)

func main() {
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	http.HandleFunc("/", www.Handle)
	http.HandleFunc("/api/slack/roll/", api.SlackRollHandler)
	http.HandleFunc("/api/dflow/", api.DialogueWebhookHandler)
	http.HandleFunc("/savecommand", queue.ProcessSaveCommand)
	appengine.Main()
}
