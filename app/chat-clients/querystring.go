package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/aasmall/dicemagic/app/dicelang/errors"
)

func QueryStringRollHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, _ := e.(*env)
	log := env.log.WithRequest(r)
	log.Debug("dial dice server")

	cmd := r.URL.Query().Get("cmd")
	prob, _ := strconv.ParseBool(r.URL.Query().Get("p"))
	diceServerResponse, err := Roll(env.diceServerClient, cmd, RollOptionWithProbability(prob))
	if err != nil {
		env.log.Errorf("Unexpected error: %+v", err)
		fmt.Fprintf(w, "response: %+v", diceServerResponse)
		return err
	}

	if diceServerResponse.Ok {
		fmt.Fprintf(w, "%+v", diceServerResponse)
	} else {
		if diceServerResponse.Error.Code == errors.Friendly {
			fmt.Fprint(w, diceServerResponse.Error.Msg)
		} else {
			fmt.Fprintf(w, "Oops! an error occured: %s", diceServerResponse.Error.Msg)
		}
	}
	return nil
}
