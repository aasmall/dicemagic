package main

import "strings"

/*
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

	//If not the expected intent, bail.
	if !strings.Contains(dialogueFlowRequest.QueryResult.Intent.Name, "b41d0bdc-45f0-4099-ac34-40baf8dbb9ec") {
		return
	}
	log.Debugf(ctx, "Confidence %d\n", dialogueFlowRequest.QueryResult.IntentDetectionConfidence)
	queryText := dialogueFlowRequest.QueryResult.QueryText
	dialogueFlowResponse.FulfillmentText = queryText
	log.Debugf(ctx, "QueryText: %#v", queryText)

	log.Debugf(ctx, "dialogueFlowRequest.QueryResult.Parameters: %#v",
		dialogueFlowRequest.QueryResult.Parameters)

	log.Debugf(ctx, "dialogueFlowRequest.QueryResult.ParametersDice: %#v",
		dialogueFlowRequest.QueryResult.Parameters["DiceExpression"])

	diceExpression := dialogueFlowRequest.QueryResult.Parameters["DiceExpression"].([]interface{})[0].(string)

	log.Debugf(ctx, "Parameters %+v\n\n", dialogueFlowRequest.QueryResult.Parameters)

	//add any missing close parens at the end
	diceExpression = addMissingCloseParens(diceExpression)
	// add ROLL identifier for parser
	if !strings.Contains(strings.ToUpper(diceExpression), "ROLL") {
		diceExpression = fmt.Sprintf("ROLL %s", diceExpression)
	}
	log.Debugf(ctx, "diceExpression: %#v",
		diceExpression)

	var text string
	stmt, err := NewParser(strings.NewReader(diceExpression)).Parse()
_, err = stmt.TotalDamage()
	log.Debugf(ctx, fmt.Sprintf("damage map: %+v", stmt))
	if err != nil {
		text = err.Error()
	} else {
		if stmt.HasDamageTypes() {
			text, err = stmt.TotalDamageString()
			text = fmt.Sprintf("You delt: \n%s", text)
		} else {
			text, err = stmt.TotalSimpleRollString()
		}
		if err != nil {
			text = err.Error()
		}
	}
	dialogueFlowResponse.FulfillmentText = text
	dialogueFlowResponse.Payload.Slack.Text = text
	log.Debugf(ctx, spew.Sprintf("My Response:\n%v", dialogueFlowResponse))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dialogueFlowResponse)

}
*/
func addMissingCloseParens(text string) string {
	if strings.Count(text, ")") < strings.Count(text, "(") {
		text += ")"
		return addMissingCloseParens(text)
	}
	return text
}
