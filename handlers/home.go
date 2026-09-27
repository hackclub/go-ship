package handlers

import "net/http"
import "html/template"
import "os"
import "bananajeanss/go-ship/db"

type PageData struct {
	HCAAuthURL string
	IsAuthed   bool
	CommitHash string
}

var commitHash = "dev"

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("./templates/index.html")
	if err != nil {
		http.Error(w, "500 internal server error", 500)
		return
	}

	clientId := os.Getenv("HCA_CLIENT_ID")
	redirectURI := os.Getenv("BASE_URL") + "/auth/callback"

	loggedIn := false
	cookie, err := r.Cookie("goship_session")
	if err == nil {
		loggedIn = db.IsLoggedIn(cookie.Value)
	}

	data := PageData{
		HCAAuthURL: "https://auth.hackclub.com/oauth/authorize?client_id=" + clientId + "&redirect_uri=" + redirectURI + "&response_type=code&scope=openid+profile+email+name+profile+slack_id+verification_status",
		IsAuthed:   loggedIn,
		CommitHash: func() string {
			if len(commitHash) > 7 {
				return commitHash[:7] // shorten to short hash if > 7 length
			}
			return commitHash
		}(),
	}

	tmpl.Execute(w, data)
}
