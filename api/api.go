package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/lnsp/dodel/models"
)

// Handler is a simple API handler.
type Handler struct {
	mux      *http.ServeMux
	database *models.JSONDatabase
	debug    bool
}

// New initializes a new API handler bound to the given database.
func New(db *models.JSONDatabase, debug bool) *Handler {
	mux := http.NewServeMux()
	api := &Handler{mux, db, debug}
	mux.HandleFunc("/api/", api.status)
	mux.HandleFunc("/api/add", api.add)
	mux.HandleFunc("/api/list", api.list)
	mux.HandleFunc("/api/show", api.show)
	mux.HandleFunc("/api/vote", api.vote)
	mux.HandleFunc("/api/karma", api.karma)
	return api
}

// ServeHTTP serves HTTP requests.
func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler.debug {
		log.Printf("api: %v\n", r.URL.String())
	}
	handler.mux.ServeHTTP(w, r)
}

// /add DATA={user, color, text, topic} -> {id}
func (handler *Handler) add(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	add := struct {
		User  string `json:"user"`
		Color string `json:"color"`
		Text  string `json:"text"`
		Topic string `json:"topic"`
	}{}
	if err := decoder.Decode(&add); err != nil {
		http.Error(w, "Failed to parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	id := handler.database.Post(add.User, add.Text, add.Color, add.Topic, nil)
	enc := json.NewEncoder(w)
	if err := enc.Encode(struct {
		ID string `json:"id"`
	}{
		ID: id,
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// list -> [JSONPost...]
func (handler *Handler) list(w http.ResponseWriter, r *http.Request) {
	posts := handler.database.List(models.DefaultListCount, models.DefaultMaxAge, models.DefaultMinRank)
	jsonPosts := handler.database.ToJSONPosts(posts)
	enc := json.NewEncoder(w)
	if err := enc.Encode(jsonPosts); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// show DATA={id} -> {JSONPost}
func (handler *Handler) show(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	req := struct {
		ID string `json:"id"`
	}{}
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Failed to decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	selected := handler.database.Get(req.ID)
	if selected.ID == "" {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}
	encoder := json.NewEncoder(w)
	comments := handler.database.Comments(selected.ID)
	jsonComments := handler.database.ToJSONPosts(comments)
	if err := encoder.Encode(struct {
		ID        string            `json:"id"`
		User      string            `json:"user"`
		Text      string            `json:"text"`
		Votes     int               `json:"votes"`
		Timestamp int64             `json:"timestamp"`
		Color     string            `json:"color"`
		Comments  []models.JSONPost `json:"comments"`
	}{
		Color:     selected.Color,
		ID:        selected.ID,
		User:      selected.UserID,
		Text:      selected.Text,
		Votes:     handler.database.CountVotes(selected.ID),
		Timestamp: selected.Timestamp.Unix(),
		Comments:  jsonComments,
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// /vote DATA={post, user, upvote} -> {votes}
func (handler *Handler) vote(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	req := struct {
		Post string `json:"post"`
		User string `json:"user"`
		Up   bool   `json:"upvote"`
	}{}
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Failed to decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	handler.database.Vote(req.User, req.Post, req.Up)
	count := handler.database.CountVotes(req.Post)
	enc := json.NewEncoder(w)
	if err := enc.Encode(struct {
		Votes int `json:"votes"`
	}{
		Votes: count,
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// /karma DATA={user} -> {karma}
func (handler *Handler) karma(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	req := struct {
		User string `json:"user"`
	}{}
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Failed to decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	karma := handler.database.Karma(req.User)
	enc := json.NewEncoder(w)
	if err := enc.Encode(struct {
		Karma int `json:"karma"`
	}{
		Karma: karma,
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// / -> service status
func (handler *Handler) status(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}
