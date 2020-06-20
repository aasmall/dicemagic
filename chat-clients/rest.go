package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	errors "github.com/aasmall/dicemagic/lib/dicelang-errors"
)

// RESTRollResponse is the Go representation of the response JSON
type RESTRollResponse struct {
	Cmd    string `json:"cmd"`
	Result string `json:"result"`
	Ok     bool   `json:"ok"`
	Err    string `json:"err,omitempty"`
}

// RESTRollRequest is the Go representation of the request JSON
type RESTRollRequest struct {
	Cmd         string `json:"cmd"`
	Chart       bool   `json:"with_chart,omitempty"`
	Probability bool   `json:"with_probability,omitempty"`
}

// RESTRollHandler handles requests to /roll
func RESTRollHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	ecm, _ := e.(*externalClientsManager)
	log := ecm.loggingClient.WithRequest(r)
	req := &RESTRollRequest{}

	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		log.Errorf("Unexpected error decoding REST request: %+v", err)
		return err
	}
	resp := &RESTRollResponse{Cmd: req.Cmd}
	diceServerResponse, err := Roll(ecm.rollerClient, req.Cmd, RollOptionWithProbability(req.Probability), RollOptionWithChart(req.Chart), RollOptionWithContext(context.TODO()), RollOptionWithTimeout(time.Second*2))
	if err != nil {
		errString := fmt.Sprintf("Unexpected error: %+v", err)
		resp.Ok = false
		resp.Err = errString
		ecm.loggingClient.Error(errString)
		return nil
	}
	if diceServerResponse.Ok {
		resp.Ok = true
		resp.Result = diceServerResponse.StringFromRollResponse()
	} else {
		if diceServerResponse.Error.Code == errors.Friendly {
			resp.Ok = true
			resp.Result = diceServerResponse.Error.Msg
		} else {
			resp.Ok = false
			resp.Err = diceServerResponse.Error.Msg
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	return nil
}
