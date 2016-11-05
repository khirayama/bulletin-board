package main

import (
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/twitter"
	"html/template"
	"log"
	"net/http"
	"os"
)

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
var db, _ = gorm.Open("sqlite3", "development.db")

const SessionName = "_bulletin_board_session"

func init() {
	goth.UseProviders(
		twitter.New(os.Getenv("TWITTER_KEY"),
			os.Getenv("TWITTER_SECRET"),
			"http://localhost:8080/auth/callback?provider=twitter"),
	)
}

func main() {
	db.AutoMigrate(&User{})

	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler)
	r.HandleFunc("/bulletin-board", bulletinBoardHandler)

	r.HandleFunc("/auth", authHandler)
	r.HandleFunc("/auth/callback", sessionCreateHandler)
	r.HandleFunc("/logout", logoutHandler)

	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}

// handlers
func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles("static/index.html")
	tmpl.Execute(w, nil)
}

func bulletinBoardHandler(w http.ResponseWriter, r *http.Request) {
	authenticate(w, r)

	tmpl, _ := template.ParseFiles("static/bulletin-board.html")
	tmpl.Execute(w, nil)
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userId := session.Values["user_id"]
	if userId == nil {
		BeginAuthHandler(w, r)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["user_id"] = nil
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusFound)
}

func sessionCreateHandler(w http.ResponseWriter, r *http.Request) {
	user, err := CompleteUserAuth(w, r)
	if err != nil {
		panic(err)
	}
	currentUser := &User{
		Provider: user.Provider,
		Uid:      user.UserID,
		Nickname: user.NickName,
		ImageUrl: user.AvatarURL,
	}
	db.Where("provider = ? AND uid = ?", user.Provider, user.UserID).Find(&currentUser)
	if db.NewRecord(currentUser) {
		db.Create(currentUser)
	}

	session, err := store.Get(r, SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session.Values["user_id"] = currentUser.ID
	session.Values["access_token"] = user.AccessToken
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusFound)
}

func authenticate(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userId := session.Values["user_id"]
	if userId == nil {
		log.Print(userId)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// models
type User struct {
	gorm.Model
	Provider string
	Uid      string
	Nickname string
	ImageUrl string
}

// auth
func BeginAuthHandler(res http.ResponseWriter, req *http.Request) {
	url := GetAuthURL(res, req)
	http.Redirect(res, req, url, http.StatusTemporaryRedirect)
}

var SetState = func(req *http.Request) string {
	state := req.URL.Query().Get("state")
	if len(state) > 0 {
		return state
	}
	return "state"
}

func GetAuthURL(res http.ResponseWriter, req *http.Request) string {
	providerName := GetProviderName(req)
	provider, _ := goth.GetProvider(providerName)
	sess, _ := provider.BeginAuth(SetState(req))
	url, _ := sess.GetAuthURL()
	session, _ := store.Get(req, SessionName)
	session.Values[SessionName] = sess.Marshal()
	session.Save(req, res)

	return url
}

var CompleteUserAuth = func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
	providerName := GetProviderName(req)
	provider, _ := goth.GetProvider(providerName)
	session, _ := store.Get(req, SessionName)
	sess, _ := provider.UnmarshalSession(session.Values[SessionName].(string))
	sess.Authorize(provider, req.URL.Query())

	return provider.FetchUser(sess)
}

var GetProviderName = getProviderName

func getProviderName(req *http.Request) string {
	provider := req.URL.Query().Get("provider")
	return provider
}
