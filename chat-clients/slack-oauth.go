package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	errors "github.com/aasmall/dicemagic/lib/dicelang-errors"
)

func SlackOAuthHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()
	ctx := r.Context()
	c, _ := e.(*SlackChatClient)
	log := c.log.WithRequest(r)

	oauthError := r.FormValue("error")
	if oauthError == "access_denied" {
		http.Redirect(w, r, c.config.slackOAuthDeniedURL, http.StatusSeeOther)
		log.Infof("OAuth Access Denied, redirecting to: %s", c.config.slackOAuthDeniedURL)
		return nil
	}

	clientSecret, err := c.Decrypt(ctx, c.config.kmsSlackKey, c.config.encSlackClientSecret)
	if err != nil {
		log.Criticalf("error decrypting secret: %s", err)
		return err
	}

	form := url.Values{}
	form.Add("code", r.FormValue("code"))
	if strings.Contains(strings.ToLower(c.config.podName), "local") {
		form.Add("redirect_uri", c.config.localRedirectURI)
	}

	req, err := http.NewRequest("POST", c.config.slackTokenURL, strings.NewReader(form.Encode()))
	req.SetBasicAuth(c.config.slackClientID, clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.traceClient.Do(req)
	if err != nil {
		log.Criticalf("error getting oAuthToken: %s", err)
		return err
	}
	defer resp.Body.Close()

	oauthResponse := &SlackInstallInstanceDoc{}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		err := errors.New("error in oAuth Response")
		log.Criticalf("%s: %s", err, string(bodyBytes))
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		return err
	}

	err = json.NewDecoder(resp.Body).Decode(oauthResponse)
	if err != nil {
		log.Criticalf("error decoding oAuth Response: %s", err)
		return err
	}

	if !oauthResponse.Ok {
		log.Criticalf("error in oAuth Response: %s", oauthResponse.Error)
		return errors.New("Error in oAuth Response: " + oauthResponse.Error)
	}

	// Encrypt and replace BotAccessToken
	encBotAccessToken, err := c.Encrypt(ctx, c.config.kmsSlackKey, oauthResponse.Bot.EncBotAccessToken)
	if err != nil {
		log.Criticalf("error encrypting BotAccessToken: %s", err)
		return err
	}
	oauthResponse.Bot.EncBotAccessToken = encBotAccessToken

	// Encrypt and replace AccessToken
	encAccessToken, err := c.Encrypt(ctx, c.config.kmsSlackKey, oauthResponse.EncAccessToken)
	if err != nil {
		log.Criticalf("error encrypting AccessToken: %s", err)
		return err
	}
	oauthResponse.EncAccessToken = encAccessToken

	SlackTeam := &SlackTeam{TeamID: oauthResponse.TeamID, TeamName: oauthResponse.TeamName, SlackAppID: c.config.slackAppID}
	log.Debugf("upserting SlackTeam: %+v", SlackTeam)
	k, err := c.UpsetSlackTeam(ctx, SlackTeam)
	if err != nil {
		log.Errorf("error upserting SlackTeam: %s", err)
		return err
	}
	log.Debugf("upserting SlackInstallInstance: %+v", oauthResponse)
	k, err = c.UpsertSlackInstallInstance(ctx, oauthResponse, k)
	if err != nil {
		log.Errorf("error upserting SlackInstallInstance: %s", err)
		return err
	}
	slackSuccessURL := fmt.Sprintf("https://slack.com/app_redirect?app=%s", c.config.slackAppID)
	http.Redirect(w, r, slackSuccessURL, http.StatusSeeOther)

	return nil
}
