package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"net/http"
	"os"
)

func slackOauthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	//log.Debugf(ctx, spew.Sprintf("oauth Request from Slack:\n%#v", r))
	r.ParseForm()
	code := r.FormValue("code")
	state := r.FormValue("state")
	var oAuthAccessRequest OauthAccessRequest
	oAuthAccessRequest.Code = code
	oAuthAccessRequest.State = state
	oAuthAccessRequest.ClientID = os.Getenv("SLACK_CLIENT_ID")
	oAuthAccessRequest.ClientSecret = slackClientSecret(ctx)
	PostOAuthAccessRequest(ctx, oAuthAccessRequest)

}
func PostOAuthAccessRequest(ctx context.Context, oAuthAccessRequest OauthAccessRequest) {
	methodUrl := "https://slack.com/api/oauth.access"
	client := urlfetch.Client(ctx)

	req, _ := http.NewRequest("GET", methodUrl, nil)

	basicAuthHeader := fmt.Sprintf("%s:%s", oAuthAccessRequest.ClientID, oAuthAccessRequest.ClientSecret)
	encodedAuthHeaderCredentials := base64.StdEncoding.EncodeToString([]byte(basicAuthHeader))
	encodedAuthHeader := fmt.Sprintf("Basic %s", encodedAuthHeaderCredentials)
	//req.SetBasicAuth(encodedClientID, encodedClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("Authorization", encodedAuthHeader)

	log.Debugf(ctx, "Authorization: %s", encodedAuthHeader)

	q := req.URL.Query()
	q.Add("code", oAuthAccessRequest.Code)

	req.URL.RawQuery = q.Encode()
	if appengine.IsDevAppServer() {
		log.Debugf(ctx, spew.Sprintf("If this were deployed, I would issue this request:\n%#v", req))
	} else {
		resp, err := client.Do(req)
		if err != nil {
			log.Criticalf(ctx, fmt.Sprintf("%+v", err))
			return
		}
		PersistIntegration(ctx, resp)
		defer resp.Body.Close()
	}

}

func PersistIntegration(ctx context.Context, resp *http.Response) {
	log.Debugf(ctx, spew.Sdump(resp.Body))
	var oAuthApprovalResponse = new(OAuthApprovalResponse)
	db, err := configureDatastoreDB(ctx, os.Getenv("PROJECT_ID"))
	if err != nil {
		log.Criticalf(ctx, "Critical Error in PersistIntegration: %+v", err)
	}
	err = json.NewDecoder(resp.Body).Decode(&oAuthApprovalResponse)
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	var integration = new(Integration)
	integration.OAuthApprovalResponse = *oAuthApprovalResponse
	id, err := db.AddIntegration(ctx, integration)
	if err != nil {
		log.Criticalf(ctx, "Could not Add Integration: %+v", err)
	}
	integration.ID = id
	log.Debugf(ctx, fmt.Sprintf("Created Integration. ID: %+v", integration.ID))
}
