package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/aasmall/dicemagic/app/dicelang/errors"
)

func SlackOAuthHandler(e interface{}, w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()
	ctx := r.Context()
	env, _ := e.(*env)
	log := env.log.WithRequest(r)

	oauthError := r.FormValue("error")
	if oauthError == "access_denied" {
		http.Redirect(w, r, env.config.slackOAuthDeniedURL, http.StatusSeeOther)
		log.Infof("OAuth Access Denied, redirecting to: %s", env.config.slackOAuthDeniedURL)
		return nil
	}

	clientSecret, err := decrypt(ctx, env, env.config.encSlackClientSecret)
	if err != nil {
		log.Criticalf("error decrypting secret: %s", err)
		return err
	}

	form := url.Values{}
	form.Add("code", r.FormValue("code"))
	if strings.Contains(strings.ToLower(env.config.podName), "local") {
		form.Add("redirect_uri", env.config.LocalRedirectUri)
	}

	req, err := http.NewRequest("POST", env.config.slackTokenURL, strings.NewReader(form.Encode()))
	req.SetBasicAuth(env.config.slackClientID, clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.traceClient.Do(req)
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
	encBotAccessToken, err := encrypt(ctx, env, oauthResponse.Bot.EncBotAccessToken)
	if err != nil {
		log.Criticalf("error encrypting BotAccessToken: %s", err)
		return err
	}
	oauthResponse.Bot.EncBotAccessToken = encBotAccessToken

	// Encrypt and replace AccessToken
	encAccessToken, err := encrypt(ctx, env, oauthResponse.EncAccessToken)
	if err != nil {
		log.Criticalf("error encrypting AccessToken: %s", err)
		return err
	}
	oauthResponse.EncAccessToken = encAccessToken

	log.Debugf("upserting SlackTeam: %+v", oauthResponse)
	k, err := upsetSlackTeam(ctx, env, &SlackTeam{TeamID: oauthResponse.TeamID, TeamName: oauthResponse.TeamName})
	if err != nil {
		log.Errorf("error upserting SlackTeam: %s", err)
		return err
	}
	k, err = upsertSlackInstallInstance(ctx, env, oauthResponse, k)
	if err != nil {
		log.Errorf("error upserting SlackInstallInstance: %s", err)
		return err
	}
	slackSuccessURL := fmt.Sprintf("https://slack.com/app_redirect?app=%s", env.config.slackAppID)
	http.Redirect(w, r, slackSuccessURL, http.StatusSeeOther)

	return nil
}
