package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var naturalLanguageRegexp = regexp.MustCompile(`(?i)^.*roll\s+(?P<expression>\d+d\d+.+)$`)
var sliceRegex = regexp.MustCompile(`(?i)[\+\-\/\*]*\d+d\d+.+?\)`)
var damageTypeRegex = regexp.MustCompile(`(?i)\((.+?)\)`)
var diceExpressionRegex = regexp.MustCompile(`(?i)[\+\-\/\*]*(\d+d\d+.*?)\(`)

func slackRoll(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	content := r.FormValue("text")
	rollResult := evaluate(parse(content))

	fmt.Fprintf(w, "You rolled %d\n", rollResult)
}

func url_verificationHandler(body []byte, w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var challengeRequest ChallengeRequest
	err := json.Unmarshal(body, &challengeRequest)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	fmt.Fprintln(w, challengeRequest.Challenge)

}
func event_callbackHandler(integration *Integration, body []byte, w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var eventCallback EventCallback
	err := json.Unmarshal(body, &eventCallback)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	if eventCallback.Token != slackVerificationToken(ctx) {
		err := fmt.Errorf("Received Token does not match VerificationToken: %+v", eventCallback.Token)
		log.Debugf(ctx, fmt.Sprintf("%s", err))
		return
	}
	switch eventCallback.InnerEvent.Type {
	case "app_mention":
		app_mentionHandler(integration, InnerEvent(eventCallback.InnerEvent), w, r, ctx)
	default:
		log.Debugf(ctx, fmt.Sprintf("Unknown Callback Event: %+v", eventCallback.InnerEvent.Type))
	}
}
func app_mentionHandler(integration *Integration,
	innerEvent InnerEvent,
	w http.ResponseWriter,
	r *http.Request,
	ctx context.Context) {
	if strings.ContainsAny(innerEvent.Text, "()") {
		naturalLanguageAttack := innerEvent.Text
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

		text := fmt.Sprintf("<@%s> delt:\n%sFor a total of %d", innerEvent.User, damageString, total)
		postTextToChannel(ctx, integration, innerEvent.Channel, text)
		fmt.Println(fmt.Sprintf("Attack: %+v", attack.totalDamage()))
	} else {
		resultOfDice := parseMentionAndRoll(innerEvent.Text)
		text := fmt.Sprintf("<@%s> rolled %d", innerEvent.User, resultOfDice)
		postTextToChannel(ctx, integration, innerEvent.Channel, text)
	}

}
func parseMentionAndRoll(text string) int64 {
	match := naturalLanguageRegexp.FindStringSubmatch(text)
	paramsMap := make(map[string]string)
	for i, name := range naturalLanguageRegexp.SubexpNames() {
		if i > 0 && i <= len(match) {
			paramsMap[name] = match[i]
		}
	}
	roll := evaluate(parse(paramsMap["expression"]))
	return roll
}
func postTextToChannel(ctx context.Context, integration *Integration, channel string, text string) {
	methodUrl := "https://slack.com/api/chat.postMessage"

	client := urlfetch.Client(ctx)

	message := new(Message)
	message.Text = text
	message.Channel = channel
	b, err := json.Marshal(message)
	req, err := http.NewRequest("POST", methodUrl, bytes.NewBuffer(b))
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", slackBotAccessToken(ctx, integration)))
	if appengine.IsDevAppServer() {
		log.Debugf(ctx, spew.Sprintf("If this were deployed, I would issue this request:\n%#v", req))
	} else {
		resp, err := client.Do(req)
		if err != nil {
			log.Criticalf(ctx, fmt.Sprintf("%+v", err))
			return
		}
		defer resp.Body.Close()
	}
}
func slackEventRouter(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	body, err := ioutil.ReadAll(r.Body)
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	defer r.Body.Close()
	//
	//parse TeamID from unstructured request
	teamID, _ := result["team_id"].(string)
	integration := IntegrationsForRequest(ctx, teamID)

	log.Debugf(ctx, "Found Integration: %+v", integration)
	//
	//route to appropriate handler for eventy type
	eventType := result["type"]
	switch eventType {
	case "url_verification":
		url_verificationHandler(body, w, r, ctx)
	case "event_callback":
		event_callbackHandler(integration, body, w, r, ctx)
	default:
	}
}
func IntegrationsForRequest(ctx context.Context, teamID string) *Integration {

	//find Integration from cloud datastore
	log.Debugf(ctx, "Looing for integrations for team_id: %s", teamID)
	db, err := configureDatastoreDB(ctx, os.Getenv("PROJECT_ID"))
	if err != nil {
		log.Criticalf(ctx, "%+v", err)
	}
	integrations, _ := db.ListIntegrationsByTeam(ctx, teamID)

	if len(integrations) < 0 {
		log.Criticalf(ctx, "No Integrations found.")
		return new(Integration)
	} else if len(integrations) > 1 {
		log.Criticalf(ctx, "More than one integration found for team. Returning first.")
		return integrations[0]
	} else {
		return integrations[0]
	}
}
func parseLanguageintoAttack(ctx context.Context, input string) *Attack {
	if !(strings.ContainsAny(input, "()")) {
		log.Criticalf(ctx, "Input does not contain damage types.")
		return nil
	}
	attack := new(Attack)
	segments := sliceRegex.FindAllString(input, -1)
	for i, s := range segments {
		attack.DamageSegment = append(attack.DamageSegment, DamageSegment{
			damagetype:     damageTypeRegex.FindStringSubmatch(s)[1],
			diceExpression: diceExpressionRegex.FindStringSubmatch(s)[1]})

		operator := diceExpressionRegex.FindStringSubmatch(s)[0][0:1]
		if !strings.ContainsAny(operator, "+/-*") {
			attack.DamageSegment[i].operator = "+"
		} else {
			attack.DamageSegment[i].operator = operator
		}

	}
	return attack
}
