package main

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/twitter"
	"html/template"
	"net/http"
	"os"
)

var store = sessions.NewCookieStore([]byte(os.Getenv("SECRET_KEY")))

func init() {
	goth.UseProviders(
		twitter.New(os.Getenv("TWITTER_KEY"),
			os.Getenv("TWITTER_SECRET"),
			"http://localhost:8080/auth/twitter/callback?provider=twitter"),
	)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler)
	r.HandleFunc("/bulletin-board", bulletinBoardHandler)

	r.HandleFunc("/auth/{provider}", gothic.BeginAuthHandler)
	r.HandleFunc("/auth/{provider}/callback", sessionCreateHandler)

	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}

// handlers
func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles("static/index.html")
	tmpl.Execute(w, nil)
}

func bulletinBoardHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles("static/bulletin-board.html")
	tmpl.Execute(w, nil)
}

func sessionCreateHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "_bulletin_board_session")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["user-id"] = 1
	session.Save(r, w)

	http.Redirect(w, r, "/bulletin-board", http.StatusFound)
}
