package main

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

func createAttachmentsDamageString(rollTotals []RollTotal) string {
	var buffer bytes.Buffer
	for _, e := range rollTotals {
		if e.rollType == "" {
			buffer.WriteString(fmt.Sprintf("%d of type _unspecified_", e.rollResult))
		} else {
			buffer.WriteString(fmt.Sprintf("%d of type %s", e.rollResult, e.rollType))
		}
	}
	return buffer.String()
}

func slackRoll(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ctx := appengine.NewContext(r)
	//Form into syntacticly correct roll statement.
	if r.FormValue("token") != slackVerificationToken(ctx) {
		fmt.Fprintf(w, "This is not the droid you're looking for.")
		return
	}
	content := fmt.Sprintf("roll %s", r.FormValue("text"))
	expression, err := NewParser(strings.NewReader(content)).Parse()
	if err != nil {
		printErrorToSlack(ctx, err, w, r)
		return
	}
	slackRollResponse := SlashRollJSONResponse{}
	attachment, err := expression.ToSlackAttachment()
	if err != nil {
		printErrorToSlack(ctx, err, w, r)
		return
	}
	slackRollResponse.Attachments = append(slackRollResponse.Attachments, attachment)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slackRollResponse)
}

type RollDecision struct {
	question string
	result   int64
	choices  []string
}

func (decision *RollDecision) ToSlackAttachment() (Attachment, error) {
	attachment := Attachment{
		Fallback:   fmt.Sprintf("I rolled 1d%d to help decide. Results are in: %s", len(decision.choices), decision.choices[decision.result]),
		AuthorName: decision.question,
		Color:      stringToColor(decision.choices[decision.result])}
	field := Field{Title: decision.choices[decision.result], Short: true}
	attachment.Fields = append(attachment.Fields, field)
	return attachment, nil
}

func (expression *RollExpression) ToSlackAttachment() (Attachment, error) {

	rollTotals, err := expression.getTotalsByType()
	attachment := Attachment{}
	if err != nil {
		return attachment, err
	}
	attachment = Attachment{
		Fallback:   createAttachmentsDamageString(rollTotals),
		AuthorName: expression.String(),
		Color:      stringToColor(expression.initialText)}
	totalRoll := int64(0)
	allUnspecified := true
	rollCount := 0
	for _, e := range rollTotals {
		if e.rollType != "" {
			allUnspecified = false
		}
		rollCount++
	}
	field := Field{}
	if allUnspecified {
		for _, e := range rollTotals {
			totalRoll += e.rollResult
		}
		field = Field{Title: fmt.Sprintf("%d", totalRoll), Short: false}
		attachment.Fields = append(attachment.Fields, field)
	} else {
		for _, e := range rollTotals {
			totalRoll += e.rollResult
			fieldTitle := e.rollType
			if e.rollType == "" {
				fieldTitle = "_Unspecified_"
			}
			field := Field{Title: fieldTitle, Value: fmt.Sprintf("%d", e.rollResult), Short: true}
			attachment.Fields = append(attachment.Fields, field)
		}
		if rollCount > 1 {
			field = Field{Title: fmt.Sprintf("For a total of: %d", totalRoll), Short: false}
			attachment.Fields = append(attachment.Fields, field)
		}
	}

	return attachment, nil
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
