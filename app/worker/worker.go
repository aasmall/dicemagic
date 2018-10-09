package main

import (
	"bytes"
	"encoding/gob"
	"net/http"

	"github.com/aasmall/dicemagic/roll"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
)

func main() {
	http.HandleFunc("/savecommand/", saveCommandHandler)
	appengine.Main()
}
func saveCommandHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	tasks, err := taskqueue.Lease(ctx, 100, "savecommand", 60)
	if err != nil {
		log.Criticalf(ctx, "Could not lease tasks: %v", err)
		return
	}
	db, err := roll.ConfigureDatastoreDB(ctx)
	if err != nil {
		log.Criticalf(ctx, "Could not Configure Datastore: %v", err.Error())
		return
	}
	for _, task := range tasks {
		var command roll.RollCommand
		buff := bytes.NewBuffer(task.Payload)
		err := gob.NewDecoder(buff).Decode(&command)
		if err != nil {
			log.Criticalf(ctx, err.Error())
			continue
		}
		err = db.UpsertRoll(ctx, command.ID, &command)
		if err != nil {
			log.Criticalf(ctx, err.Error())
			continue
		}
		log.Infof(ctx, "successfully saved command with id %s", command.ID)
		taskqueue.Delete(ctx, task, "savecommand")
	}
}
