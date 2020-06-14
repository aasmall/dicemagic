package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	inAddress = ":8080"
	projectID = "k8s-dice-magic"
)

func main() {
	var env string
	flag.StringVar(&env, "environment", "prod", "Can be either 'local', 'dev', or 'prod'. Controls which directory to load from.")

	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	serve404 := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
		http.ServeFile(w, r, "/srv/404.html")
	}

	// Define inbound Routes
	var srvDir http.Dir
	switch env {
	case "local":
		srvDir = http.Dir("/srv/local")
	case "dev":
		srvDir = http.Dir("/srv/dev")
	case "prod":
		srvDir = http.Dir("/srv/prod")
	default:
		log.Fatalln("env not supplied as arg and I didn't use the default for some dumb reason.")
	}

	fileHandler := http.StripPrefix("/", CustomFileServer(srvDir, serve404))
	log.Printf("serving from: %v\n", env)
	// h := &ochttp.Handler{Handler: fileHandler, StartOptions: trace.StartOptions{SpanKind: trace.SpanKindServer}, FormatSpanName: formatSpanName}

	// Define a server with timeouts
	srv := &http.Server{
		Addr:         inAddress,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      fileHandler,
		// Handler:      ochttp.WithRouteTag(h, "www/"), // Pass our instance of gorilla/mux and tracer in.
	}
	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	}()

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	os.Exit(0)

}

type customFileServer struct {
	root            http.Dir
	NotFoundHandler func(http.ResponseWriter, *http.Request)
}

// CustomFileServer serves static content, disables directory browsing
// and calls NotFoundHandler in the case of a 404
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
	defer f.Close()
	s, _ := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(name, "/") + "/index.html"
		g, err := os.Open(index)
		if err != nil {
			fs.NotFoundHandler(w, r)
			return
		}
		defer g.Close()
	}

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
