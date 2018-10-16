package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/aasmall/dicemagic/app/dice-server/dicelang"
	"github.com/aasmall/dicemagic/app/dice-server/roll"
	"google.golang.org/appengine"
)

//SlackRollJSONResponse is the response format for slack slash commands
type SlackRollJSONResponse struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
}
type Attachment struct {
	Pretext    string  `json:"pretext"`
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
	if r.FormValue("token") != roll.SlackVerificationToken(ctx) {
		fmt.Fprintf(w, "This is not the droid you're looking for.")
		return
	}
	content := r.FormValue("text")
	stmts, _, err := dicelang.NewParser(content).Statements()
	if err != nil {
		printErrorToSlack(ctx, err, w, r)
		return
	}
	slackRollResponse := SlackRollJSONResponse{}
	attachments, err := createSlackAttachments(stmts...)
	if err != nil {
		printErrorToSlack(ctx, err, w, r)
		return
	}
	slackRollResponse.Attachments = append(slackRollResponse.Attachments, attachments...)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slackRollResponse)
}

func rollDecisionToSlackAttachment(decision *roll.RollDecision) (Attachment, error) {
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

func createSlackAttachments(stmts ...*dicelang.AST) ([]Attachment, error) {
	var attachments []Attachment
	for _, stmt := range stmts {
		var fields []Field
		total, dice, err := stmt.GetDiceSet()
		if err != nil {
			return nil, err
		}
		var faces []interface{} //will be variadic
		for _, d := range dice.Dice {
			faces = append(faces, dicelang.FacesSliceString(d.Faces))
		}
		for k, v := range dice.TotalsByColor {
			var fieldTitle string
			if k == "" && len(dice.TotalsByColor) == 1 {
				fieldTitle = ""
			} else if k == "" {
				fieldTitle = "Unspecified"
			} else {
				fieldTitle = k
			}
			field := Field{Title: fieldTitle, Value: strconv.FormatFloat(v, 'f', -1, 64), Short: true}
			fields = append(fields, field)
		}
		attachment := Attachment{
			Fallback:   dicelang.TotalsMapString(dice.TotalsByColor),
			AuthorName: fmt.Sprintf(stmt.String(), faces...),
			Color:      stringToColor(stmt.String())}
		if len(dice.Dice) > 1 {
			field := Field{Title: "Total", Value: strconv.FormatFloat(total, 'f', -1, 64), Short: false}
			fields = append(fields, field)
		}
		attachment.Fields = append(attachment.Fields, fields...)
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func printErrorToSlack(ctx context.Context, err error, w http.ResponseWriter, r *http.Request) {
	slackRollResponse := new(SlackRollJSONResponse)
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
