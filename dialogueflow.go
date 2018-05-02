package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

type DialogueFlowRequest struct {
	ResponseID                  string                      `json:"responseId"`
	QueryResult                 DialogueFlowQueryResult     `json:"queryResult"`
	OriginalDetectIntentRequest OriginalDetectIntentRequest `json:"originalDetectIntentRequest"`
	Session                     string                      `json:"session"`
}

type DialogueFlowQueryResult struct {
	QueryText                string                 `json:"queryText"`
	Action                   string                 `json:"action"`
	Parameters               map[string]interface{} `json:"parameters"`
	AllRequiredParamsPresent bool                   `json:"allRequiredParamsPresent"`
	Intent                   struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"intent"`
	IntentDetectionConfidence float64 `json:"intentDetectionConfidence"`
	DiagnosticInfo            struct {
	} `json:"diagnosticInfo"`
	LanguageCode string `json:"languageCode"`
}
type OriginalDetectIntentRequest struct {
	Payload struct {
		Data struct {
			AuthedUsers []string `json:"authed_users"`
			EventID     string   `json:"event_id"`
			APIAppID    string   `json:"api_app_id"`
			TeamID      string   `json:"team_id"`
			Event       struct {
				EventTs string `json:"event_ts"`
				Channel string `json:"channel"`
				Text    string `json:"text"`
				Type    string `json:"type"`
				User    string `json:"user"`
				Ts      string `json:"ts"`
			} `json:"event"`
			Type      string  `json:"type"`
			EventTime float64 `json:"event_time"`
			Token     string  `json:"token"`
		} `json:"data"`
		Source string `json:"source"`
	} `json:"payload"`
}
type DialogueFlowQueryResult2 struct {
	QueryText                string                 `json:"queryText"`
	Parameters               map[string]interface{} `json:"parameters"`
	AllRequiredParamsPresent bool                   `json:"allRequiredParamsPresent"`
	FulfillmentText          string                 `json:"fulfillmentText"`
	FulfillmentMessages      []struct {
		Text struct {
			Text []string `json:"text"`
		} `json:"text"`
	} `json:"fulfillmentMessages"`
	OutputContexts []struct {
		Name          string `json:"name"`
		LifespanCount int    `json:"lifespanCount"`
		Parameters    struct {
			Param string `json:"param"`
		} `json:"parameters"`
	} `json:"outputContexts"`
	Intent struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"intent"`
	IntentDetectionConfidence float64 `json:"intentDetectionConfidence"`
	DiagnosticInfo            struct {
	} `json:"diagnosticInfo"`
	LanguageCode string `json:"languageCode"`
}
type DialogueFlowParameter struct {
	name  string
	value string
}
type DialogueFlowResponse struct {
	FulfillmentText     string `json:"fulfillmentText"`
	FulfillmentMessages []struct {
		Card struct {
			Title    string `json:"title"`
			Subtitle string `json:"subtitle"`
			ImageURI string `json:"imageUri"`
			Buttons  []struct {
				Text     string `json:"text"`
				Postback string `json:"postback"`
			} `json:"buttons"`
		} `json:"card"`
	} `json:"fulfillmentMessages"`
	Source  string `json:"source"`
	Payload struct {
		Slack SlashRollJSONResponse `json:"slack"`
	} `json:"payload"`
	OutputContexts []struct {
		Name          string `json:"name"`
		LifespanCount int    `json:"lifespanCount"`
		Parameters    struct {
			Param string `json:"param"`
		} `json:"parameters"`
	} `json:"outputContexts"`
	FollowupEventInput struct {
		Name         string `json:"name"`
		LanguageCode string `json:"languageCode"`
		Parameters   struct {
			Param string `json:"param"`
		} `json:"parameters"`
	} `json:"followupEventInput"`
}

func dialogueWebhookHandler(w http.ResponseWriter, r *http.Request) {
	//response := "This is a sample response from your webhook!"
	ctx := appengine.NewContext(r)

	// Save a copy of this request for debugging.
	//requestDump, err := httputil.DumpRequest(r, true)
	//if err != nil {
	//		log.Criticalf(ctx, "%v", err)
	//		return
	//	}
	//	log.Debugf(ctx, "Whole Request: %s", string(requestDump))

	//read body into dialogueFlowRequest
	var dialogueFlowRequest = new(DialogueFlowRequest)
	dialogueFlowResponse := new(DialogueFlowResponse)
	err := json.NewDecoder(r.Body).Decode(dialogueFlowRequest)
	defer r.Body.Close()
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}

	//If not the expected intent, bail.
	if !strings.Contains(dialogueFlowRequest.QueryResult.Intent.Name, "b41d0bdc-45f0-4099-ac34-40baf8dbb9ec") {
		//TODO: proper error handling to client
		return
	}

	queryText := dialogueFlowRequest.QueryResult.QueryText

	//log a bunch of crap
	log.Debugf(ctx, "Confidence %d\n", dialogueFlowRequest.QueryResult.IntentDetectionConfidence)
	log.Debugf(ctx, "Parameters %+v\n", dialogueFlowRequest.QueryResult.Parameters)
	log.Debugf(ctx, "QueryText: %#v", queryText)
	log.Debugf(ctx, "dialogueFlowRequest.QueryResult.Parameters: %#v",
		dialogueFlowRequest.QueryResult.Parameters)
	log.Debugf(ctx, "dialogueFlowRequest.QueryResult.ParametersDice: %#v",
		dialogueFlowRequest.QueryResult.Parameters["DiceExpression"])

	diceExpressionString := addMissingCloseParens(dialogueFlowRequest.QueryResult.Parameters["DiceExpression"].([]interface{})[0].(string))

	// add ROLL identifier for parser
	if !strings.Contains(strings.ToUpper(diceExpressionString), "ROLL") {
		diceExpressionString = fmt.Sprintf("roll %s", diceExpressionString)
	}
	log.Debugf(ctx, "diceExpression: %#v", diceExpressionString)

	expression, err := NewParser(strings.NewReader(diceExpressionString)).Parse()
	expression.initialText = diceExpressionString
	if err != nil {
		log.Criticalf(ctx, "%v", err)
		return
	}

	dialogueFlowResponse.FulfillmentText = ""
	dialogueFlowResponse.Payload.Slack, err = expression.ToSlackRollResponse()
	if err != nil {
		log.Criticalf(ctx, "%v", err)
		return
	}
	log.Debugf(ctx, spew.Sprintf("My Response:\n%+v", dialogueFlowResponse.Payload.Slack))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dialogueFlowResponse)
}

func addMissingCloseParens(text string) string {
	if strings.Count(text, ")") < strings.Count(text, "(") {
		text += ")"
		return addMissingCloseParens(text)
	}
	if strings.Count(text, "]") < strings.Count(text, "[") {
		text += "]"
		return addMissingCloseParens(text)
	}
	return text
}
