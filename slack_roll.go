package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudkms/v1"
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

func slackOauthToken(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_KEY"), ctx)
}
func slackClientSecret(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_CLIENT_SECRET"), ctx)
}
func slackBotAccessToken(ctx context.Context) string {
	return decrypt(os.Getenv("SLACK_BOT_USER_ACCES_TOKEN"), ctx)
}

func decrypt(ciphertext string, ctx context.Context) string {
	projectID := os.Getenv("PROJECT_ID")
	keyRing := os.Getenv("KMSKEYRING")
	key := os.Getenv("KMSKEY")
	locationID := "global"

	client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}

	kmsService, err := cloudkms.New(client)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	parentName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, locationID, keyRing, key)
	req := &cloudkms.DecryptRequest{
		Ciphertext: ciphertext,
	}
	resp, err := kmsService.Projects.Locations.KeyRings.CryptoKeys.Decrypt(parentName, req).Do()
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
	}
	decodedString, _ := base64.StdEncoding.DecodeString(resp.Plaintext)
	return string(decodedString)
}

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
func event_callbackHandler(body []byte, w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var eventCallback EventCallback
	err := json.Unmarshal(body, &eventCallback)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	if eventCallback.Token != slackClientSecret(ctx) {
		err := fmt.Errorf("Received Token does not match ClientSecret: %+v", eventCallback.Token)
		log.Debugf(ctx, fmt.Sprintf("%s", err))
		return
	}
	switch eventCallback.InnerEvent.Type {
	case "app_mention":
		app_mentionHandler(InnerEvent(eventCallback.InnerEvent), w, r, ctx)
	default:
		log.Debugf(ctx, fmt.Sprintf("Unknown Callback Event: %+v", eventCallback.InnerEvent.Type))
	}
}
func app_mentionHandler(innerEvent InnerEvent,
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

		text := fmt.Sprintf("<@%s> delt:\n %sFor a total of %d", innerEvent.User, damageString, total)
		postTextToChannel(ctx, innerEvent.Channel, text)
		fmt.Println(fmt.Sprintf("Attack: %+v", attack.totalDamage()))
	} else {
		resultOfDice := parseMentionAndRoll(innerEvent.Text)
		text := fmt.Sprintf("<@%s> rolled %d", innerEvent.User, resultOfDice)
		postTextToChannel(ctx, innerEvent.Channel, text)
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
func postTextToChannel(ctx context.Context, channel string, text string) {
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", slackBotAccessToken(ctx)))
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
	eventType := result["type"]
	switch eventType {
	case "url_verification":
		url_verificationHandler(body, w, r, ctx)
	case "event_callback":
		event_callbackHandler(body, w, r, ctx)
	default:
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
