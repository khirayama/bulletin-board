package main

import (
	"github.com/gorilla/mux"
	"net/http"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/sessions", SessionHandler)
	r.HandleFunc("/bulletin-board", DashboardHandler)
	http.Handle("/", r)
}
