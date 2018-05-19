package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aasmall/dicemagic/api"
	"github.com/aasmall/dicemagic/queue"
	"go.opencensus.io/trace"
	"google.golang.org/appengine"
)

func main() {
	serveIndexHtml := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "www/public/404.html")
	}
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	http.Handle("/", http.StripPrefix("/", CustomFileServer(http.Dir("www/public"), serveIndexHtml)))
	http.HandleFunc("/api/slack/roll/", api.SlackRollHandler)
	http.HandleFunc("/api/dflow/", api.DialogueWebhookHandler)
	http.HandleFunc("/savecommand", queue.ProcessSaveCommand)
	appengine.Main()
}

type customFileServer struct {
	root            http.Dir
	NotFoundHandler func(http.ResponseWriter, *http.Request)
}

func CustomFileServer(root http.Dir, NotFoundHandler http.HandlerFunc) http.Handler {
	return &customFileServer{root: root, NotFoundHandler: NotFoundHandler}
}

func (fs *customFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if containsDotDot(r.URL.Path) {
		http.Error(w, "URL should not contain '/../' parts", http.StatusBadRequest)
		return
	}

	//if empty, set current directory
	dir := string(fs.root)
	if dir == "" {
		dir = "."
	}

	//add prefix and clean
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	upath = path.Clean(upath)

	//path to file
	name := path.Join(dir, filepath.FromSlash(upath))

	//check if file exists
	f, err := os.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			fs.NotFoundHandler(w, r)
			return
		}
	}
	s, err := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(name, "/") + "/index.html"
		if _, err := os.Open(index); err != nil {
			fs.NotFoundHandler(w, r)
			return
		}
	}
	defer f.Close()

	http.ServeFile(w, r, name)
}
func containsDotDot(v string) bool {
	if !strings.Contains(v, "..") {
		return false
	}
	for _, ent := range strings.FieldsFunc(v, isSlashRune) {
		if ent == ".." {
			return true
		}
	}
	return false
}

func isSlashRune(r rune) bool { return r == '/' || r == '\\' }
