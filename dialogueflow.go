package main

import (
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
)

func dialogueWebhookHandler(w http.ResponseWriter, r *http.Request) {
	//response := "This is a sample response from your webhook!"
	ctx := appengine.NewContext(r)

	// Save a copy of this request for debugging.
	requestDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Criticalf(ctx, "%v", err)
	}
	log.Debugf(ctx, "Whole Request: %s", string(requestDump))

	body, err := ioutil.ReadAll(r.Body)
	var dialogueFlowRequest DialogueFlowRequest
	err = json.Unmarshal(body, &dialogueFlowRequest)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	defer r.Body.Close()
	var dialogueFlowResponse = new(DialogueFlowResponse)

	if !strings.Contains(dialogueFlowRequest.QueryResult.Intent.Name, "b41d0bdc-45f0-4099-ac34-40baf8dbb9ec") {
		return
	}
	log.Debugf(ctx, "Confidence %d\n", dialogueFlowRequest.QueryResult.IntentDetectionConfidence)
	queryText := dialogueFlowRequest.QueryResult.QueryText
	dialogueFlowResponse.FulfillmentText = queryText
	log.Debugf(ctx, "QueryText: %#v", queryText)

	//diceExpression := dialogueFlowRequest.QueryResult.Parameters["DiceExpression"][0]
	log.Debugf(ctx, "dialogueFlowRequest.QueryResult.Parameters: %#v",
		dialogueFlowRequest.QueryResult.Parameters)

	log.Debugf(ctx, "dialogueFlowRequest.QueryResult.ParametersDice: %#v",
		dialogueFlowRequest.QueryResult.Parameters["DiceExpression"])

	diceExpression := dialogueFlowRequest.QueryResult.Parameters["DiceExpression"].([]interface{})[0].(string)
	log.Debugf(ctx, "diceExpression: %#v",
		diceExpression)

	log.Debugf(ctx, "Parameters %+v\n\n", dialogueFlowRequest.QueryResult.Parameters)

	if strings.Count(diceExpression, ")") < strings.Count(diceExpression, "(") {
		//TODO: move to recursive function
		diceExpression += ")"
	}

	var text string
	if strings.ContainsAny(diceExpression, "()") {
		naturalLanguageAttack := diceExpression
		var attack = parseLanguageintoAttack(ctx, naturalLanguageAttack)
		m := attack.totalDamage()
		var damageString string
		log.Debugf(ctx, fmt.Sprintf("Map:%#v", m))
		var total int64
		for k := range m {
			damageString += fmt.Sprintf("%d %s damage\n", m[k], k)
			total += m[k]
		}

		log.Debugf(ctx, fmt.Sprintf("damagestring:%s", damageString))

		text = fmt.Sprintf("%s delt:\n%sFor a total of %d", "you", damageString, total)

	} else {
		resultOfDice := evaluate(parse(diceExpression))
		text = fmt.Sprintf("%s rolled %d", "you", resultOfDice)
	}

	dialogueFlowResponse.FulfillmentText = text
	dialogueFlowResponse.Payload.Slack.Text = text
	log.Debugf(ctx, spew.Sprintf("My Response:\n%v", dialogueFlowResponse))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dialogueFlowResponse)
	//
	//parse TeamID from unstructured request

}
