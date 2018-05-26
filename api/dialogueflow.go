package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"

	"github.com/aasmall/dicemagic/roll"
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
	Payload             struct {
		Slack  SlackRollJSONResponse `json:"slack,omitempty"`
		Google AssistantResponse     `json:"google,omitempty"`
	} `json:"payload,omitempty"`
	OutputContexts []struct {
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
	case "roll":
		handleRollIntent(ctx, *dialogueFlowRequest, w, r)
	case "decide":
		handleDecideIntent(ctx, *dialogueFlowRequest, w, r)
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
	dialogueFlowResponse.Payload.Slack = slackRollResponse

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
	dialogueFlowResponse := new(DialogueFlowResponse)

	slackRollResponse := SlackRollJSONResponse{}

	googleAssistantRollResponse := AssistantResponse{}
	googleAssistantRollResponse.RichResponse = RichResponse{}

	var formattedText bytes.Buffer

	diceExpressionCount := len(command.RollExpresions)
	t := int64(0)
	for i := 0; i < diceExpressionCount; i++ {
		// Roll all the dice!
		err := command.RollExpresions[i].Total()
		if err != nil {
			printErrorToDialogFlow(ctx, err, w, r)
			return
		}
		// Populate generic Fulfillment Messages
		fulfillmentMessage := rollExpressionToFulfillmentMessage(&command.RollExpresions[i])
		attachment, err := rollExpressionToSlackAttachment(&command.RollExpresions[i])
		if err != nil {
			printErrorToDialogFlow(ctx, err, w, r)
			return
		}
		dialogueFlowResponse.FulfillmentMessages = append(dialogueFlowResponse.FulfillmentMessages, fulfillmentMessage)
		slackRollResponse.Attachments = append(slackRollResponse.Attachments, attachment)
		markdownRow, loopTotal, err := rollExpressionToMarkdown(&command.RollExpresions[i])
		formattedText.WriteString(markdownRow)
		if i != diceExpressionCount-1 {
			formattedText.WriteString("  \n---  \n")
		}
		t += loopTotal
	}

	simpleResponseItem := SimpleResponseItem{}
	if diceExpressionCount > 1 {
		simpleResponseItem.SimpleResponse.TextToSpeech = "Rolling."
	} else {
		simpleResponseItem.SimpleResponse.TextToSpeech = fmt.Sprintf("You rolled %d", t)
	}
	googleAssistantRollResponse.RichResponse.Items = append(googleAssistantRollResponse.RichResponse.Items, simpleResponseItem)

	basicCardItem := BasicCardItem{}
	basicCardItem.BasicCard.Title = "results"
	basicCardItem.BasicCard.FormattedText = formattedText.String()
	googleAssistantRollResponse.RichResponse.Items = append(googleAssistantRollResponse.RichResponse.Items, basicCardItem)

	dialogueFlowResponse.FulfillmentText = formattedText.String()
	dialogueFlowResponse.Payload.Google = googleAssistantRollResponse
	dialogueFlowResponse.Payload.Slack = slackRollResponse

	log.Debugf(ctx, "RichResponse: %+v", googleAssistantRollResponse.RichResponse)
	//Send Response
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(dialogueFlowResponse)
}

func handleDecideIntent(ctx context.Context, dialogueFlowRequest DialogueFlowRequest, w http.ResponseWriter, r *http.Request) {

	dialogueFlowResponse := new(DialogueFlowResponse)
	slackRollResponse := SlackRollJSONResponse{}

	//create a RollDecision and fill it
	rollDecision := roll.RollDecision{}
	rollDecision.Question = dialogueFlowRequest.QueryResult.QueryText

	dflowChoices := dialogueFlowRequest.QueryResult.Parameters["Choices"].([]interface{})

	if len(dflowChoices) < 2 {
		rollDecision.Choices = append(rollDecision.Choices, "Yes")
		rollDecision.Choices = append(rollDecision.Choices, "No")
	} else {
		for _, v := range dflowChoices {
			rollDecision.Choices = append(rollDecision.Choices, strings.Title(v.(string)))
		}
		log.Debugf(ctx, fmt.Sprintf("Choices(%d): %v", len(rollDecision.Choices), rollDecision.Choices))
	}
	d := roll.Dice{NumberOfDice: int64(1), Sides: int64(len(rollDecision.Choices))}
	result, err := d.Roll()
	if err != nil {
		log.Errorf(ctx, "Couldn't roll dice: %v", err)
		return
	}
	rollDecision.Result = result - 1

	log.Debugf(ctx, fmt.Sprintf("RollDecision:\n%+v", rollDecision))

	//create a slack attachment from RollDecision
	attachment, _ := rollDecisionToSlackAttachment(&rollDecision)
	//attach it to Slack payload
	slackRollResponse.Attachments = append(slackRollResponse.Attachments, attachment)
	slackRollResponse.Text = "I'll roll some dice to help you make that decision."
	dialogueFlowResponse.Payload.Slack = slackRollResponse
	//log.Debugf(ctx, spew.Sprintf("My Response:\n%+v", dialogueFlowResponse))
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
		diceExpressionString := addMissingCloseParens(dialogueFlowRequest.QueryResult.Parameters["DiceExpression"].([]interface{})[i].(string))
		// add ROLL identifier for parser
		if !strings.Contains(strings.ToUpper(diceExpressionString), "ROLL") {
			diceExpressionString = fmt.Sprintf("roll %s", diceExpressionString)
		}
		diceStrings = append(diceStrings, diceExpressionString)
	}
	command.FromString(diceStrings...)

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

func rollExpressionToFulfillmentMessage(expression *roll.RollExpression) DialogFlowFulfillmentMessage {
	returnMessage := DialogFlowFulfillmentMessage{}
	returnMessage.DialogFlowCard.Title = ""

	//Dice rolls into Expanded, formatted string
	var fmtString []interface{}
	for i, d := range expression.DiceSet.Dice {
		fmtString = append(fmtString, fmt.Sprintf("%dd%d(%d)", d.NumberOfDice, d.Sides, expression.DiceSet.Results[i]))
	}
	returnMessage.DialogFlowCard.Title = fmt.Sprintf(expression.ExpandedTextTemplate, fmtString...)
	var buff bytes.Buffer
	rollTotal := int64(0)
	for i, t := range expression.RollTotals {
		rollTotal += t.RollResult
		buff.WriteString(strconv.FormatInt(t.RollResult, 10))
		buff.WriteString(" [")
		buff.WriteString(t.RollType)
		buff.WriteString("]")
		if i != len(expression.RollTotals) {
			buff.WriteString("\n")
		} else {
			buff.WriteString("\nFor a total of: ")
			buff.WriteString(strconv.FormatInt(rollTotal, 10))
		}
	}
	returnMessage.DialogFlowCard.Subtitle = expression.TotalsString()
	return returnMessage
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
	return text
}
