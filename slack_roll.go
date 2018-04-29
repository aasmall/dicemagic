package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

//SlashRollJSONResponse is the response format for slack slash commands
type SlashRollJSONResponse struct {
	Text        string `json:"text"`
	Tttachments struct {
		Text string `json:"text"`
	} `json:"attachments"`
}

func slackRoll(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ctx := appengine.NewContext(r)
	var responseText string
	//Form into syntacticly correct roll statement.
	if r.FormValue("token") != slackVerificationToken(ctx) {
		fmt.Fprintf(w, "This is not the droid you're looking for.")
		return
	}
	content := fmt.Sprintf("ROLL %s", r.FormValue("text"))
	stmt, err := NewParser(strings.NewReader(content)).Parse()
	if err != nil {
		responseText = err.Error()
	} else {
		if stmt.HasDamageTypes() {
			responseText, err = stmt.TotalDamageString()
		} else {
			responseText, err = stmt.TotalSimpleRollString()
		}
		if err != nil {
			responseText = err.Error()
		}
	}
	log.Debugf(ctx, fmt.Sprintf("%+v", stmt))
	slackRollResponse := new(SlashRollJSONResponse)
	slackRollResponse.Text = responseText
	slackRollResponse.Tttachments.Text = responseText
	log.Debugf(ctx, fmt.Sprintf("%+v", slackRollResponse))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slackRollResponse)
}
