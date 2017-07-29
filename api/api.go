package api

import (
	"encoding/json"
	"net/http"

	"google.golang.org/appengine"

	"zwig/models"
)

const (
	APIVersion = "v1.0.0"
)

// Handler is a simple API handler.
type Handler struct {
	mux *http.ServeMux
}

// New initializes a new API handler bound to the given database.
func New() *Handler {
	mux := http.NewServeMux()
	api := &Handler{mux}
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
	handler.mux.ServeHTTP(w, r)
}

// /add DATA={user, color, text, topic} -> {id}
func (handler *Handler) add(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	decoder := json.NewDecoder(r.Body)
	add := struct {
		Author string `json:"user"`
		Color  string `json:"color"`
		Text   string `json:"text"`
		Parent int64  `json:"topic"`
	}{}
	if err := decoder.Decode(&add); err != nil {
		http.Error(w, "Failed to parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	id, err := models.SubmitPost(c, add.Author, add.Text, add.Color, add.Parent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(struct {
		ID int64 `json:"id"`
	}{
		ID: id,
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// list -> [JSONPost...]
func (handler *Handler) list(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	posts, ids, err := models.TopPosts(c, 30, -10.0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonPosts, err := models.ToJSONComments(c, posts, ids)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(jsonPosts); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// show DATA={id} -> {JSONPost}
func (handler *Handler) show(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	dec := json.NewDecoder(r.Body)
	req := struct {
		ID int64 `json:"id"`
	}{}
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Failed to decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	post, err := models.GetPost(c, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	numVotes, err := models.NumberOfVotes(c, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	encoder := json.NewEncoder(w)
	comments, ids, err := models.GetComments(c, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonComments, err := models.ToJSONComments(c, comments, ids)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := encoder.Encode(struct {
		ID       int64             `json:"id"`
		Author   string            `json:"user"`
		Text     string            `json:"text"`
		Votes    int               `json:"votes"`
		Date     int64             `json:"timestamp"`
		Color    string            `json:"color"`
		Comments []models.JSONPost `json:"comments"`
	}{
		Color:    post.Color,
		ID:       req.ID,
		Author:   post.Author,
		Text:     post.Text,
		Votes:    numVotes,
		Date:     post.Date.Unix(),
		Comments: jsonComments,
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// /vote DATA={post, user, upvote} -> {votes}
func (handler *Handler) vote(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	dec := json.NewDecoder(r.Body)
	req := struct {
		Post   int64  `json:"post"`
		Author string `json:"user"`
		Upvote bool   `json:"upvote"`
	}{}
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Failed to decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	_, err := models.SubmitVote(c, req.Author, req.Post, req.Upvote)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	numVotes, err := models.NumberOfVotes(c, req.Post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(struct {
		Votes int `json:"votes"`
	}{
		Votes: numVotes,
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

// /karma DATA={user} -> {karma}
func (handler *Handler) karma(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	dec := json.NewDecoder(r.Body)
	req := struct {
		User string `json:"user"`
	}{}
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Failed to decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	karma := models.GetKarma(c, req.User)
	enc := json.NewEncoder(w)
	if err := enc.Encode(struct {
		Karma int `json:"karma"`
	}{
		Karma: karma,
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

func (handler *Handler) status(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(APIVersion))
}
