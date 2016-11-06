package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/twitter"
	"html/template"
	"io/ioutil"
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
	db.AutoMigrate(&User{}, &Message{})

	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler)
	r.HandleFunc("/bulletin-board", bulletinBoardHandler)

	r.HandleFunc("/auth", authHandler)
	r.HandleFunc("/auth/callback", sessionCreateHandler)
	r.HandleFunc("/logout", logoutHandler)

	r.HandleFunc("/api/v1/messages", messagesHandler).Methods("GET")
	r.HandleFunc("/api/v1/messages", messageCreateHandler).Methods("POST")

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
		gothic.BeginAuthHandler(w, r)
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
	user, err := gothic.CompleteUserAuth(w, r)
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

// api handlers
func messagesHandler(w http.ResponseWriter, r *http.Request) {
	authenticate(w, r)

	session, err := store.Get(r, SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var currentUser User
	db.First(&currentUser, session.Values["user_id"])

	var messages []Message
	db.Find(&messages)

	// for response
	type ResponseUser struct {
		Id       uint   `json:"id"`
		Provider string `json:"provider"`
		Uid      string `json:"uid"`
		Nickname string `json:"nickname"`
		ImageUrl string `json:"imageUrl"`
	}

	type ResponseMessage struct {
		Id     uint         `json:"id"`
		Text   string       `json:"text"`
		Author ResponseUser `json:"author"`
	}

	var responseMessages []ResponseMessage
	for _, message := range messages {
		var user User
		db.First(&user, message.UserID)

		responseMessage := &ResponseMessage{
			Id:   message.ID,
			Text: message.Text,
			Author: *&ResponseUser{
				Id:       user.ID,
				Provider: user.Provider,
				Uid:      user.Uid,
				Nickname: user.Nickname,
				ImageUrl: user.ImageUrl,
			},
		}

		responseMessages = append(responseMessages, *responseMessage)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responseMessages)
}

func messageCreateHandler(w http.ResponseWriter, r *http.Request) {
	authenticate(w, r)

	session, err := store.Get(r, SessionName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var user User
	db.First(&user, session.Values["user_id"])

	var message Message

	body, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &message)
	message.UserID = user.ID

	db.Model(&user).Create(&message)

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(message)
}

// models
type User struct {
	gorm.Model
	Provider string
	Uid      string
	Nickname string
	ImageUrl string

	Messages []Message
}

type Message struct {
	gorm.Model
	UserID uint
	Text   string
}
