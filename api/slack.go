package api

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net/http"
	"strings"

	"github.com/aasmall/dicemagic/lib"
	"google.golang.org/appengine"
)

//SlashRollJSONResponse is the response format for slack slash commands
type SlashRollJSONResponse struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
}
type Attachment struct {
	Fallback   string  `json:"fallback"`
	Color      string  `json:"color"`
	AuthorName string  `json:"author_name"`
	Fields     []Field `json:"fields"`
}
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func SlackRollHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ctx := appengine.NewContext(r)
	//Form into syntacticly correct roll statement.
	if r.FormValue("token") != slackVerificationToken(ctx) {
		fmt.Fprintf(w, "This is not the droid you're looking for.")
		return
	}
	content := fmt.Sprintf("roll %s", r.FormValue("text"))
	expression, err := lib.NewParser(strings.NewReader(content)).Parse()
	if err != nil {
		printErrorToSlack(ctx, err, w, r)
		return
	}
	slackRollResponse := SlashRollJSONResponse{}
	attachment, err := rollExpressionToSlackAttachment(expression)
	if err != nil {
		printErrorToSlack(ctx, err, w, r)
		return
	}
	slackRollResponse.Attachments = append(slackRollResponse.Attachments, attachment)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slackRollResponse)
}

func rollDecisionToSlackAttachment(decision *lib.RollDecision) (Attachment, error) {
	attachment := Attachment{
		Fallback: fmt.Sprintf("I rolled 1d%d to help decide. Results are in: %s",
			len(decision.Choices),
			decision.Choices[decision.Result]),
		AuthorName: decision.Question,
		Color:      stringToColor(decision.Choices[decision.Result])}
	field := Field{Title: decision.Choices[decision.Result], Short: true}
	attachment.Fields = append(attachment.Fields, field)
	return attachment, nil
}

func rollExpressionToSlackAttachment(expression *lib.RollExpression) (Attachment, error) {
	rollTotals, err := expression.GetTotalsByType()
	attachment := Attachment{}
	if err != nil {
		return attachment, err
	}
	attachment = Attachment{
		Fallback:   createAttachmentsDamageString(rollTotals),
		AuthorName: expression.String(),
		Color:      stringToColor(expression.InitialText)}
	totalRoll := int64(0)
	allUnspecified := true
	rollCount := 0
	for _, e := range rollTotals {
		if e.RollType != "" {
			allUnspecified = false
		}
		rollCount++
	}
	field := Field{}
	if allUnspecified {
		for _, e := range rollTotals {
			totalRoll += e.RollResult
		}
		field = Field{Title: fmt.Sprintf("%d", totalRoll), Short: false}
		attachment.Fields = append(attachment.Fields, field)
	} else {
		for _, e := range rollTotals {
			totalRoll += e.RollResult
			fieldTitle := e.RollType
			if e.RollType == "" {
				fieldTitle = "_Unspecified_"
			}
			field := Field{Title: fieldTitle, Value: fmt.Sprintf("%d", e.RollResult), Short: true}
			attachment.Fields = append(attachment.Fields, field)
		}
		if rollCount > 1 {
			field = Field{Title: fmt.Sprintf("For a total of: %d", totalRoll), Short: false}
			attachment.Fields = append(attachment.Fields, field)
		}
	}

	return attachment, nil
}
func createAttachmentsDamageString(rollTotals []lib.RollTotal) string {
	var buffer bytes.Buffer
	for _, e := range rollTotals {
		if e.RollType == "" {
			buffer.WriteString(fmt.Sprintf("%d ", e.RollResult))
		} else {
			buffer.WriteString(fmt.Sprintf("%d of type %s ", e.RollResult, e.RollType))
		}
	}
	return buffer.String()
}
func printErrorToSlack(ctx context.Context, err error, w http.ResponseWriter, r *http.Request) {
	slackRollResponse := new(SlashRollJSONResponse)
	slackRollResponse.Text = err.Error()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slackRollResponse)
}
func stringToColor(input string) string {
	bi := big.NewInt(0)
	h := md5.New()
	h.Write([]byte(input))
	hexb := h.Sum(nil)
	hexstr := hex.EncodeToString(hexb[:len(hexb)/2])
	bi.SetString(hexstr, 16)
	rand.Seed(bi.Int64())
	r := rand.Intn(0xff)
	g := rand.Intn(0xff)
	b := rand.Intn(0xff)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}
