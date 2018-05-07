package queue

import (
	"net/http"

	"github.com/aasmall/dicemagic/internal"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
)

//ProcessSaveCommand recieves requests to save memcache entries to
//persistant storage
func ProcessSaveCommand(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	r.ParseForm()
	log.Debugf(ctx, "Saving Command")
	var command internal.RollCommand
	_, err := memcache.Gob.Get(ctx, r.FormValue("key"), &command)
	if err != nil {
		log.Criticalf(ctx, "Failed to get key (%s): %s", r.FormValue("key"), err)
		return
	}

	if command.DiceMagicDatabase == nil {
		db, err := internal.ConfigureDatastoreDB(ctx)
		if err != nil {
			log.Criticalf(ctx, err.Error())
			return
		}
		command.DiceMagicDatabase = db
	}
	err = command.DiceMagicDatabase.UpsertRoll(ctx, r.FormValue("key"), &command)
	if err != nil {
		log.Criticalf(ctx, err.Error())
		return
	}

}
