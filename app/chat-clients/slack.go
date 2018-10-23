package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net/http"
	"strconv"

	"cloud.google.com/go/datastore"
	pb "github.com/aasmall/dicemagic/app/proto"
	"github.com/nlopes/slack"
)

type SlackInstallInstanceDoc struct {
	Key            *datastore.Key `datastore:"__key__"`
	EncAccessToken string         `datastore:",noindex" json:"access_token"`
	Bot            struct {
		EncBotAccessToken string `datastore:",noindex" json:"bot_access_token"`
		BotUserID         string `datastore:",noindex" json:"bot_user_id"`
	} `datastore:",noindex" json:"bot"`
	Ok       bool   `datastore:",noindex" json:"ok"`
	Error    string `datastore:",noindex" json:"error"`
	Scope    string `datastore:",noindex" json:"scope"`
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name"`
	UserID   string `json:"user_id"`
}

//SlackRollJSONResponse is the response format for slack commands
type SlackRollJSONResponse struct {
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments"`
}

type SlackAttachment struct {
	Pretext    string       `json:"pretext"`
	Fallback   string       `json:"fallback"`
	Color      string       `json:"color"`
	AuthorName string       `json:"author_name"`
	Fields     []SlackField `json:"fields"`
}
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func returnErrorToSlack(text string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SlackRollJSONResponse{Text: text})
}

func SlackAttachmentFromRollResponse(ds *pb.DiceSet) slack.Attachment {
	retSlackAttachment := slack.Attachment{}
	var faces []interface{}
	for _, d := range ds.Dice {
		faces = append(faces, FacesSliceString(d.Faces))
	}

	for k, v := range ds.TotalsByColor {
		var fieldTitle string
		if k == "" && len(ds.TotalsByColor) == 1 {
			fieldTitle = ""
		} else if k == "" {
			fieldTitle = "Unspecified"
		} else {
			fieldTitle = k
		}
		field := slack.AttachmentField{Title: fieldTitle, Value: strconv.FormatFloat(v, 'f', -1, 64), Short: true}
		retSlackAttachment.Fields = append(retSlackAttachment.Fields, field)
	}
	retSlackAttachment.Fallback = TotalsMapString(ds.TotalsByColor)
	retSlackAttachment.AuthorName = fmt.Sprintf(ds.ReString, faces...)
	retSlackAttachment.Color = stringToColor(ds.ReString)

	if len(ds.Dice) > 1 {
		field := slack.AttachmentField{Title: "Total", Value: strconv.FormatInt(ds.Total, 10), Short: false}
		retSlackAttachment.Fields = append(retSlackAttachment.Fields, field)
	}

	return retSlackAttachment
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
