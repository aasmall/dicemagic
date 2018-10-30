package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aasmall/dicemagic/app/dicelang/errors"
	pb "github.com/aasmall/dicemagic/app/proto"
)

type RESTRollResponse struct {
	Cmd    string `json:"cmd"`
	Result string `json:"result"`
	Ok     bool   `json:"ok"`
	Err    string `json:"err"`
}
type RESTRollRequest struct {
	Cmd         string `json:"cmd"`
	Chart       bool   `json:"with_chart,omitempty"`
	Probability bool   `json:"with_probability,omitempty"`
}

func RESTRollHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, _ := e.(*env)
	log := env.log.WithRequest(r)
	req := &RESTRollRequest{}

	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		log.Errorf("Unexpected error decoding REST request: %+v", err)
		return err
	}
	resp := &RESTRollResponse{Cmd: req.Cmd}
	diceServerResponse, err := Roll(env.diceServerClient, req.Cmd, RollOptionWithProbability(req.Probability), RollOptionWithChart(req.Chart))
	if err != nil {
		errString := fmt.Sprintf("Unexpected error: %+v", err)
		resp.Ok = false
		resp.Err = errString
		env.log.Error(errString)
		return nil
	}
	if diceServerResponse.Ok {
		resp.Ok = true
		resp.Result = StringFromRollResponse(diceServerResponse)
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

func StringFromRollResponse(rr *pb.RollResponse) string {
	var s []string
	for _, ds := range rr.DiceSets {
		var faces []interface{}
		for _, d := range ds.Dice {
			faces = append(faces, facesSliceString(d.Faces))
		}
		s = append(s, fmt.Sprintf("%s = *%s*", fmt.Sprintf(ds.ReString, faces...), strconv.FormatInt(ds.Total, 10)))
	}
	if len(rr.DiceSets) > 1 {
		s = append(s, fmt.Sprintf("Total: %s", strconv.FormatInt(rr.DiceSet.Total, 10)))
	}
	return strings.Join(s, "\n")
}
