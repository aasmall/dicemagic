package main

import (
	"fmt"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"html/template"
	"net/http"
)

type Page struct {
	Title string
	Body  []byte
}

func rootHandle(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	t, err := template.ParseFiles("root.html")
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	p := Page{Title: "Welcome"}
	t.Execute(w, p)
}
