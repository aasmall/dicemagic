package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"cloud.google.com/go/datastore"
)

type SlackInstallInstance struct {
	AccessToken string `json:"access_token"`
	Bot         struct {
		BotAccessToken string `json:"bot_access_token"`
		BotUserID      string `json:"bot_user_id"`
	} `json:"bot"`
	Ok       bool   `json:"ok"`
	Scope    string `json:"scope"`
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name"`
	UserID   string `json:"user_id"`
}

type SlackInstallInstanceDoc struct {
	Key         *datastore.Key `datastore:"__key__"`
	AccessToken string
	Bot         struct {
		EncBotAccessToken string
		EncBotUserID      string
	}
	Scope    string
	TeamID   string
	TeamName string
	UserID   string
}

func SlackOAuthHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ctx := r.Context()
	slackTokenURL := "https://slack.com/api/oauth.access"

	oauthError := r.FormValue("error")
	if oauthError == "access_denied" {
		fmt.Fprintf(w, "access denied.")
		return
	}

	code := r.FormValue("code")
	clientSecret, err := decrypt(ctx, encSlackClientSecret)
	if err != nil {
		logger.Fatalf("error decrypting secret: %s", err)
	}

	form := url.Values{}
	form.Add("code", code)

	req, err := http.NewRequest("POST", slackTokenURL, strings.NewReader(form.Encode()))
	req.SetBasicAuth(slackClientID, clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := traceClient.Do(req)
	if err != nil {
		logger.Fatalf("error getting oAuthToken: %s", err)
	}
	defer resp.Body.Close()
	var oauthResponse SlackInstallInstance
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		logger.Printf("error in oAuth Response: %s", bodyString)
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&oauthResponse)
	if err != nil {
		log.Fatalf("error decoding oAuth Response: %s", err)
	}

	EncBotAccessToken, err := encrypt(ctx, oauthResponse.Bot.BotAccessToken)
	if err != nil {
		log.Fatalf("error encrypting BotAccessToken: %s", err)
	}
	EncAccessToken, err := encrypt(ctx, oauthResponse.AccessToken)
	if err != nil {
		log.Fatalf("error encrypting AccessToken: %s", err)
	}

	newDoc := &SlackInstallInstanceDoc{
		AccessToken: EncAccessToken,
		Bot: struct {
			EncBotAccessToken string
			EncBotUserID      string
		}{
			EncBotAccessToken: EncBotAccessToken,
			EncBotUserID:      oauthResponse.Bot.BotUserID,
		},
		Scope:    oauthResponse.Scope,
		TeamID:   oauthResponse.TeamID,
		TeamName: oauthResponse.TeamName,
		UserID:   oauthResponse.UserID,
	}

	var docs []SlackInstallInstanceDoc
	q := datastore.NewQuery("SlackInstallInstance").Filter("TeamID =", newDoc.TeamID).Filter("UserID =", newDoc.UserID).Limit(1)
	_, err = dsClient.GetAll(ctx, q, &docs)
	if err != nil {
		log.Fatalf("error querying for duplicate instances: %s", err)
	}
	if len(docs) > 0 {
		fmt.Fprintf(w, "Sorry, you already installed this.\n")
		fmt.Fprintf(w, "%+v", docs)
	} else {
		k, err := dsClient.Put(ctx, datastore.IncompleteKey("SlackInstallInstance", nil), newDoc)
		if err != nil {
			log.Fatalf("error inserting SlackInstallInstanceDoc: %s", err)
		}
		fmt.Fprintf(w, "Done! \nKey: %+v\n\n%+v", k, newDoc)
	}
}
