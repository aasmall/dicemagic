package main

import (
	"encoding/json"
	"net/http"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

func parseHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	//Decode request into ParseRequest type
	parseRequest := new(ParseRequest)
	json.NewDecoder(r.Body).Decode(parseRequest)
	defer r.Body.Close()
	//Prepare Response Object
	parseResponse := new(ParseResponse)

	//Call Parser and inject response into response object
	parsedString, err := parseString(ctx, parseRequest.Text)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		parseResponse.Text = err.Error()
	} else {
		parseResponse.Text = parsedString
	}
	//Encode response into response stream
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parseResponse)
}
