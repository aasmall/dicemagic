package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aasmall/dicemagic/internal/dicelang/errors"
	"github.com/aasmall/dicemagic/internal/handler"
	"github.com/nlopes/slack"
)

func SlackSlashRollHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	c, _ := e.(*SlackChatClient)

	//r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	if !c.ValidateSlackSignature(r) {
		return handler.StatusError{
			Code: http.StatusUnauthorized,
			Err:  errors.New("Invalid Slack Signature"),
		}
	}

	//log := c.log.WithRequest(r)
	s, err := slack.SlashCommandParse(r)
	if err != nil {
		fmt.Fprintf(w, "could not parse slash command: %s", err)
	}
	rollResponse, err := Roll(c.diceClient, s.Text)
	if err != nil {
		c.log.Errorf("Unexpected error: %+v", err)
		returnErrorToSlack(fmt.Sprintf("Oops! an unexpected error occured: %s", err), w, r)
		return nil
	}
	if !rollResponse.Ok {
		if rollResponse.Error.Code == errors.Friendly {
			returnErrorToSlack(rollResponse.Error.Msg, w, r)
			return nil
		} else {
			returnErrorToSlack(fmt.Sprintf("Oops! an error occured: %s", rollResponse.Error.Msg), w, r)
			return nil
		}
	}
	webhookMessage := slack.Msg{}
	webhookMessage.Attachments = append(webhookMessage.Attachments, SlackAttachmentsFromRollResponse(rollResponse)...)
	c.log.Errorf("Webook Message Sending: %+v", webhookMessage)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhookMessage)
	return nil
}
