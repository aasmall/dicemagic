package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aasmall/dicemagic/app/dicelang/errors"
	"github.com/aasmall/dicemagic/app/handler"
	pb "github.com/aasmall/dicemagic/app/proto"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

func SlackSlashRollHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	env, _ := e.(*env)

	//r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	if !ValidateSlackSignature(env, r) {
		return handler.StatusError{
			Code: http.StatusUnauthorized,
			Err:  errors.New("Invalid Slack Signature"),
		}
	}

	log := env.log.WithRequest(r)
	//read body and reset request
	s, err := slack.SlashCommandParse(r)
	if err != nil {
		fmt.Fprintf(w, "could not parse slash command: %s", err)
	}
	rollerClient := pb.NewRollerClient(env.diceServerClient)
	timeOutCtx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	cmd := s.Text
	log.Debug(cmd)
	diceServerResponse, err := rollerClient.Roll(timeOutCtx, &pb.RollRequest{Cmd: cmd})
	if err != nil {
		env.log.Errorf("Unexpected error: %+v", err)
		returnErrorToSlack(fmt.Sprintf("Oops! an unexpected error occured: %s", err), w, r)
		return err
	}

	if !diceServerResponse.Ok {
		if diceServerResponse.Error.Code == errors.Friendly {
			returnErrorToSlack(diceServerResponse.Error.Msg, w, r)
			return nil
		} else {
			returnErrorToSlack(fmt.Sprintf("Oops! an error occured: %s", diceServerResponse.Error.Msg), w, r)
			return nil

		}
	}

	webhookMessage := slack.WebhookMessage{}
	webhookMessage.Attachments = append(webhookMessage.Attachments, SlackAttachmentFromRollResponse(diceServerResponse.DiceSet))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhookMessage)
	return nil
}
