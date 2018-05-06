package www

import (
	"fmt"
	"html/template"
	"net/http"

	"os"
	"path/filepath"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

// Page Represents a simple HTML Page
type Page struct {
	Title string
	Body  []byte
}

//Handle handles incoming requests for the website
func Handle(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	paths := make(map[string]string, 0)
	err := filepath.Walk("www/templates/", func(path string, info os.FileInfo, err error) error {
		log.Debugf(ctx, "file: %s", info.Name())
		log.Debugf(ctx, "path: %s", path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != "" && path != "" {
			paths[info.Name()] = path
		}
		return nil
	})
	if err != nil {
		log.Criticalf(ctx, fmt.Sprintf("%+v", err))
		return
	}
	log.Debugf(ctx, "requested path: %s", r.URL.Path[1:])
	if r.URL.Path[1:] == "/" || r.URL.Path[1:] == "" {
		t, err := template.ParseFiles("www/templates/root.html")
		if err != nil {
			log.Criticalf(ctx, fmt.Sprintf("%+v", err))
			return
		}
		p := Page{Title: "Welcome"}
		t.Execute(w, p)
	} else {
		t, err := template.ParseFiles(paths[r.URL.Path[1:]])
		if err != nil {
			log.Criticalf(ctx, fmt.Sprintf("%+v", err))
			return
		}
		p := Page{Title: r.URL.Path[1:]}
		t.Execute(w, p)
	}
}
