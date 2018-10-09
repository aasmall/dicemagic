package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/aasmall/dicemagic/app/dice-server/dicelang"

	"github.com/aasmall/dicemagic/app/dice-server/roll"
	"go.opencensus.io/trace"

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
type DialogueFlowParameter struct {
	name  string
	value string
}
type DialogFlowFulfillmentMessage struct {
	DialogFlowCard DialogFlowCard `json:"card,omitempty"`
}
type DialogFlowCard struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	ImageURI string `json:"imageUri"`
	Buttons  []struct {
		Text     string `json:"text"`
		Postback string `json:"postback"`
	} `json:"buttons"`
}
type DialogueFlowResponse struct {
	FulfillmentText     string                         `json:"fulfillmentText,omitempty"`
	FulfillmentMessages []DialogFlowFulfillmentMessage `json:"fulfillmentMessages,omitempty"`
	Source              string                         `json:"source,omitempty"`
	Payload             *DialogFlowPayload             `json:"payload,omitempty"`
	OutputContexts      []struct {
		Name          string `json:"name,omitempty"`
		LifespanCount int    `json:"lifespanCount,omitempty"`
		Parameters    struct {
			Param string `json:"param,omitempty"`
		} `json:"parameters,omitempty"`
	} `json:"outputContexts,omitempty"`
	FollowupEventInput struct {
		Name         string `json:"name,omitempty"`
		LanguageCode string `json:"languageCode,omitempty"`
		Parameters   struct {
			Param string `json:"param,omitempty"`
		} `json:"parameters,omitempty"`
	} `json:"followupEventInput,omitempty"`
}
type DialogFlowPayload struct {
	Slack  *SlackRollJSONResponse `json:"slack,omitempty"`
	Google *AssistantResponse     `json:"google,omitempty"`
}

//AssistantResponse represents a response that will be sent to Dialogflow for Google Actions API
type AssistantResponse struct {
	ExpectUserResponse bool          `json:"expectUserResponse,omitempty"`
	IsSsml             bool          `json:"isSsml,omitempty"`
	NoInputPrompts     []interface{} `json:"noInputPrompts,omitempty"`
	RichResponse       `json:"richResponse,omitempty"`
}
type RichResponse struct {
	Items       []interface{} `json:"items,omitempty"`
	Suggestions []struct {
		Title string `json:"title,omitempty"`
	} `json:"suggestions,omitempty"`
}
type SimpleResponseItem struct {
	SimpleResponse struct {
		TextToSpeech string `json:"textToSpeech"`
		DisplayText  string `json:"displayText"`
	} `json:"simpleResponse,omitempty"`
}

type BasicCardItem struct {
	BasicCard struct {
		Title         string `json:"title,omitempty"`
		FormattedText string `json:"formattedText,omitempty"`
	} `json:"basicCard,omitempty"`
}

func DialogueWebhookHandler(w http.ResponseWriter, r *http.Request) {
	//response := "This is a sample response from your webhook!"
	ctx := appengine.NewContext(r)
	ctx, span := trace.StartSpan(ctx, "DialogueWebhookHandler")
	defer span.End()
	// Save a copy of this request for debugging.
	if strings.Contains(strings.ToLower(r.Host), "dev") || appengine.IsDevAppServer() {
		requestDump, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Criticalf(ctx, "%v", err)
			return
		}
		log.Debugf(ctx, "Whole Request: %s", string(requestDump))
	}
	//read body into dialogueFlowRequest
	var dialogueFlowRequest = new(DialogueFlowRequest)
	err := json.NewDecoder(r.Body).Decode(dialogueFlowRequest)
	defer r.Body.Close()
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	//log a bunch of crap
	log.Debugf(ctx, "Confidence %d\n", dialogueFlowRequest.QueryResult.IntentDetectionConfidence)
	log.Debugf(ctx, "Parameters %+v\n", dialogueFlowRequest.QueryResult.Parameters)
	log.Debugf(ctx, "QueryText: %#v", dialogueFlowRequest.QueryResult.QueryText)
	log.Debugf(ctx, "dialogueFlowRequest.QueryResult.Parameters: %#v",
		dialogueFlowRequest.QueryResult.Parameters)
	log.Debugf(ctx, "dialogueFlowRequest.QueryResult.ParametersDice: %#v",
		dialogueFlowRequest.QueryResult.Parameters["DiceExpression"])

	//switch on Intent
	switch strings.ToLower(dialogueFlowRequest.QueryResult.Intent.DisplayName) {
	case "roll", "interactive - roll":
		handleRollIntent(ctx, *dialogueFlowRequest, w, r)
	case "decide":
		//handleDecideIntent(ctx, *dialogueFlowRequest, w, r)
		handleDefaultIntent(ctx, *dialogueFlowRequest, w, r)
	case "command":
		handleCommandIntent(ctx, *dialogueFlowRequest, w, r)
	case "remember":
		handleRememberIntent(ctx, *dialogueFlowRequest, w, r)
	default:
		handleDefaultIntent(ctx, *dialogueFlowRequest, w, r)
	}
}

func handleDefaultIntent(ctx context.Context, dialogueFlowRequest DialogueFlowRequest, w http.ResponseWriter, r *http.Request) {
	dialogueFlowResponse := new(DialogueFlowResponse)
	dialogueFlowResponse.FulfillmentText = fmt.Sprintf("Unrecognized Intent: %s", dialogueFlowRequest.QueryResult.Intent.DisplayName)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dialogueFlowResponse)
}

func handleRememberIntent(ctx context.Context, dialogueFlowRequest DialogueFlowRequest, w http.ResponseWriter, r *http.Request) {
	dialogueFlowResponse := new(DialogueFlowResponse)
	slackRollResponse := SlackRollJSONResponse{}
	diceExpressionCount := len(dialogueFlowRequest.QueryResult.Parameters["DiceExpression"].([]interface{}))
	var command roll.RollCommand
	var diceStrings []string
	for i := 0; i < diceExpressionCount; i++ {
		diceExpressionString := addMissingCloseParens(dialogueFlowRequest.QueryResult.Parameters["DiceExpression"].([]interface{})[i].(string))
		// add ROLL identifier for parser
		if !strings.Contains(strings.ToUpper(diceExpressionString), "ROLL") {
			diceExpressionString = fmt.Sprintf("roll %s", diceExpressionString)
		}
		diceStrings = append(diceStrings, diceExpressionString)
	}
	//Parse strings into RollCommmand
	command.FromString(diceStrings...)

	//enqueue task to save last command
	namespace := dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.TeamID
	commandName := "!" + dialogueFlowRequest.QueryResult.Parameters["Command"].(string)
	command.ID = roll.HashStrings(commandName, namespace, dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.Event.User)

	err := command.Save(ctx)
	if err != nil {
		printErrorToDialogFlow(ctx, err, w, r)
		return
	}
	log.Debugf(ctx, "command:%s user: %s key: %s", commandName, dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.Event.User, command.ID)
	var attachment Attachment
	attachment.AuthorName = fmt.Sprintf("Saved %s", commandName)
	slackRollResponse.Attachments = append(slackRollResponse.Attachments, attachment)
	payload := new(DialogFlowPayload)
	payload.Slack = &slackRollResponse
	dialogueFlowResponse.Payload = payload

	//Send Response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dialogueFlowResponse)
}

func handleCommandIntent(ctx context.Context, dialogueFlowRequest DialogueFlowRequest, w http.ResponseWriter, r *http.Request) {
	commandString := dialogueFlowRequest.QueryResult.QueryText
	var rollCommand roll.RollCommand
	key := roll.HashStrings(commandString,
		dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.TeamID,
		dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.Event.User)
	err := rollCommand.Get(ctx, key)

	log.Debugf(ctx, "command:%s user: %s key: %s", commandString, dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.Event.User, key)
	if err != nil {
		printErrorToDialogFlow(ctx, err, w, r)
		return
	}
	rollCommand.ID = roll.HashStrings("!!",
		dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.TeamID,
		dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.Event.User)
	err = rollCommand.Save(ctx)
	if err != nil {
		log.Errorf(ctx, "could not persist command: %s", err)
	}
	handleRollCommand(ctx, rollCommand, w, r)

}
func handleRollCommand(ctx context.Context, command roll.RollCommand, w http.ResponseWriter, r *http.Request) {
	//DialogFlow API Wrapper
	dialogueFlowResponse := new(DialogueFlowResponse)

	//Custom Slack Payload
	slackRollResponse := SlackRollJSONResponse{}
	attachments, err := createSlackAttachments(command.RollExpresions...)
	if err != nil {
		printErrorToDialogFlow(ctx, err, w, r)
		return
	}
	slackRollResponse.Attachments = attachments
	payload := new(DialogFlowPayload)
	payload.Slack = &slackRollResponse
	dialogueFlowResponse.Payload = payload

	//Generic response
	fulfulmentText, err := createDialogFlowFulfillmentText(command.RollExpresions...)
	if err != nil {
		printErrorToDialogFlow(ctx, err, w, r)
		return
	}
	dialogueFlowResponse.FulfillmentText = fulfulmentText

	//Send Response
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(dialogueFlowResponse)
}
func handleRollIntent(ctx context.Context, dialogueFlowRequest DialogueFlowRequest, w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(ctx, "handleRollIntent")
	defer span.End()
	diceExpressionCount := len(dialogueFlowRequest.QueryResult.Parameters["DiceExpression"].([]interface{}))
	var command roll.RollCommand
	var diceStrings []string
	for i := 0; i < diceExpressionCount; i++ {
		diceExpressionString := html.UnescapeString(addMissingCloseParens(dialogueFlowRequest.QueryResult.Parameters["DiceExpression"].([]interface{})[i].(string)))
		// add ROLL identifier for parser
		if !strings.Contains(strings.ToUpper(diceExpressionString), "ROLL") {
			diceExpressionString = fmt.Sprintf("roll %s", diceExpressionString)
		}
		diceStrings = append(diceStrings, diceExpressionString)
	}
	command.FromString(html.UnescapeString(dialogueFlowRequest.QueryResult.QueryText))

	//Save for replay
	command.ID = roll.HashStrings("!!",
		dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.TeamID,
		dialogueFlowRequest.OriginalDetectIntentRequest.Payload.Data.Event.User)

	err := command.Save(ctx)
	if err != nil {
		log.Errorf(ctx, "could not persist command: %s", err)
	}
	handleRollCommand(ctx, command, w, r)

}

func createDialogFlowFulfillmentText(stmts ...*dicelang.AST) (string, error) {
	var b [][]byte
	var t float64
	for _, stmt := range stmts {
		total, dice, err := stmt.GetDiceSet()
		if err != nil {
			return "", err
		}
		var faces []interface{} //will be variadic
		for _, d := range dice.Dice {
			faces = append(faces, dicelang.FacesSliceString(d.Faces))
		}
		b = append(b, []byte(fmt.Sprintf(stmt.String(), faces...)))
		t += total
	}

	return string(bytes.Join(b, []byte("\n"))), nil
}

func printErrorToDialogFlow(ctx context.Context, err error, w http.ResponseWriter, r *http.Request) {
	dialogueFlowResponse := new(DialogueFlowResponse)
	dialogueFlowResponse.FulfillmentText = err.Error()
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
	if strings.Count(text, "}") < strings.Count(text, "{") {
		text += "}"
		return addMissingCloseParens(text)
	}
	return text
}
