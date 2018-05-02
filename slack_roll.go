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
	"google.golang.org/appengine/log"
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

func createAttachmentsDamageString(collapsedRoll map[string]int64) string {
	var buffer bytes.Buffer
	for k, v := range collapsedRoll {
		if k == "" {
			buffer.WriteString(fmt.Sprintf("%d of type _unspecified_", v))
		} else {
			buffer.WriteString(fmt.Sprintf("%d of type %s", v, k))
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
	content := fmt.Sprintf("ROLL %s", r.FormValue("text"))
	expression, err := NewParser(strings.NewReader(content)).Parse()
	if err != nil {
		printErrorToSlack(ctx, err, w, r)
		return
	}
	damageMap, err := expression.getTotalsByType()
	if err != nil {
		printErrorToSlack(ctx, err, w, r)
		return
	}

	slackRollResponse := new(SlashRollJSONResponse)
	attachment := Attachment{
		Fallback:   createAttachmentsDamageString(damageMap),
		AuthorName: fmt.Sprintf("/roll %s", r.FormValue("text")),
		Color:      stringToColor(r.FormValue("user_id"))}
	totalRoll := int64(0)
	for k, v := range damageMap {
		totalRoll += v
		fieldTitle := k
		if k == "" {
			fieldTitle = "_Unspecified_"
		}
		field := Field{Title: fieldTitle, Value: fmt.Sprintf("%d", v), Short: true}
		attachment.Fields = append(attachment.Fields, field)
		log.Debugf(ctx, fmt.Sprintf("Attachment: %+v", attachment))
	}

	field := Field{Title: fmt.Sprintf("For a total of: %d", totalRoll), Short: false}
	attachment.Fields = append(attachment.Fields, field)

	slackRollResponse.Attachments = append(slackRollResponse.Attachments, attachment)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slackRollResponse)
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
	return fmt.Sprintf("#%X%X%X", r, g, b)
}
