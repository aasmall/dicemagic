package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/aasmall/dicemagic/app/dicelang/errors"

	"golang.org/x/net/context"

	pb "github.com/aasmall/dicemagic/app/proto"
)

func QueryStringRollHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, _ := e.(*env)
	log := env.log.WithRequest(r)
	log.Info("dial dice server")
	rollerClient := pb.NewRollerClient(env.diceServerClient)
	timeOutCtx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	cmd := r.URL.Query().Get("cmd")
	prob, _ := strconv.ParseBool(r.URL.Query().Get("p"))
	diceServerResponse, err := rollerClient.Roll(timeOutCtx, &pb.RollRequest{Cmd: cmd, Probabilities: prob})
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
func sortProbMap(m map[int64]float64) []int64 {
	var keys []int64
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
