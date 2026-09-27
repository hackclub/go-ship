package main

import (
	"bananajeanss/go-ship/StartTime"
	"bananajeanss/go-ship/db"
	"bananajeanss/go-ship/handlers"
	"bananajeanss/go-ship/slack"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

func notFoundHandler(w http.ResponseWriter) {
	tmpl, err := template.ParseFiles("./templates/404.html")
	// if no template, just return 404
	if err != nil {
		http.Error(w, "404 not found :(", 404)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	tmpl.Execute(w, nil)
}

func dynamicHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	var templatePath string
	if path == "/" {
		handlers.HomeHandler(w, r)
		return
	} else {
		templatePath = fmt.Sprintf("./templates%s/index.html", path)
	}

	// no path traversal
	cleanPath := filepath.Clean(templatePath)
	if !strings.HasPrefix(cleanPath, filepath.Clean("./templates")) {
		notFoundHandler(w)
		return
	}

	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		notFoundHandler(w)
		return
	}

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, "500 internal server error", 500)
		return
	}

	tmpl.Execute(w, nil)
}

var Port = "3000"

func init() {
	// set start time
	StartTime.SetStartTime()

	// env vars
	godotenv.Load()
	Port = os.Getenv("PORT")
	if Port == "" {
		Port = "3000"
	}

	// handle ctrl+c gracefully to not get an error
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("\nShutting down...")
		os.Exit(0)
	}()
}

func main() {
	// init db
	if err := db.Init(); err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		return
	}

	// init slack bot
	slack.Init()

	// serve public static files
	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("./public"))))

	// favicon
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./public/favicon.ico")
	})

	// specific handlers
	http.HandleFunc("/auth/callback", handlers.AuthCallbackHandler)
	http.HandleFunc("/stats", handlers.StatsHandler)
	http.HandleFunc("/generatemeaidea", handlers.GenerateMeAIdeaHandler)
	http.HandleFunc("/guides", handlers.GuidesHandler)
	http.HandleFunc("/format", handlers.FormatHandler)
	http.HandleFunc("/run", handlers.RunHandler)
	http.HandleFunc("/scripts", handlers.ScriptsHandler)
	http.HandleFunc("/joinchannel", handlers.PostAddToChannelHandler)
	http.HandleFunc("/events-endpoint", slack.EventsEndpoint)

	// catch-all
	http.HandleFunc("/", dynamicHandler)

	// listen and serve
	fmt.Printf("Listening & Serving on http://localhost:%s\n", Port)
	if err := http.ListenAndServe(":"+Port, nil); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
