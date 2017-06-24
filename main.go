package main

import (
	"encoding/json"
	"fmt"
	"image"
	"log"
	"net/http"
	"time"

	"github.com/pborman/uuid"
)

type Post struct {
	ID        string
	UserID    string
	Text      string
	Color     string
	Image     image.Image
	Timestamp time.Time
}

type Vote struct {
	PostID    string
	UserID    string
	Upvote    bool
	Timestamp time.Time
}

var (
	posts = []Post{}
	votes = []Vote{}
)

func vote(user, post string, upvote bool) {
	votes = append(votes, Vote{
		PostID:    post,
		UserID:    user,
		Upvote:    upvote,
		Timestamp: time.Now(),
	})
}

func post(user, text, color string, img image.Image) string {
	id := uuid.New()
	posts = append(posts, Post{
		ID:        id,
		UserID:    user,
		Text:      text,
		Color:     color,
		Image:     img,
		Timestamp: time.Now(),
	})
	return id
}

func list() []Post {
	results := []Post{}
	for _, p := range posts {
		if time.Since(p.Timestamp) > time.Hour*24 {
			continue
		}
		results = append(results, p)
	}
	return results
}

func countVotes(id string) int {
	voted := map[string]bool{}
	count := 0
	for _, v := range votes {
		user := v.UserID
		if _, ok := voted[user]; ok {
			continue
		}
		voted[user] = true
		if v.Upvote {
			count++
		} else {
			count--
		}
	}
	return count
}

func addPost(w http.ResponseWriter, r *http.Request) {
	log.Println("/add")

	decoder := json.NewDecoder(r.Body)
	add := struct {
		User  string `json:"user"`
		Color string `json:"color"`
		Text  string `json:"text"`
	}{}
	if err := decoder.Decode(&add); err != nil {
		http.Error(w, "Failed to parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	id := post(add.User, add.Text, add.Color, nil)

	buf, err := json.Marshal(struct {
		ID string `json:"id"`
	}{id})
	if err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf)
}

func listPosts(w http.ResponseWriter, r *http.Request) {
	log.Println("/list")

	type postJSON struct {
		ID        string `json:"id"`
		User      string `json:"user"`
		Text      string `json:"text"`
		Votes     int    `json:"votes"`
		Timestamp int64  `json:"timestamp"`
	}

	results := list()
	resultsJSON := []postJSON{}
	for _, p := range results {
		resultsJSON = append(resultsJSON, postJSON{
			ID:        p.ID,
			User:      p.UserID,
			Text:      p.Text,
			Votes:     countVotes(p.ID),
			Timestamp: p.Timestamp.Unix(),
		})
	}

	if buf, err := json.Marshal(resultsJSON); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(buf)
	}
}

func showPost(w http.ResponseWriter, r *http.Request) {
	log.Println("/show")

	decoder := json.NewDecoder(r.Body)
	id := struct {
		ID string `json:"id"`
	}{}
	if err := decoder.Decode(&id); err != nil {
		http.Error(w, "Failed to decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	var selected Post
	for _, p := range posts {
		if p.ID == id.ID {
			selected = p
			break
		}
	}
	if selected.ID == "" {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(struct {
		ID        string `json:"id"`
		User      string `json:"user"`
		Text      string `json:"text"`
		Votes     int    `json:"votes"`
		Timestamp int64  `json:"timestamp"`
	}{
		ID:        selected.ID,
		User:      selected.UserID,
		Text:      selected.Text,
		Votes:     countVotes(selected.ID),
		Timestamp: selected.Timestamp.Unix(),
	}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func votePost(w http.ResponseWriter, r *http.Request) {
	log.Println("/vote")

	decoder := json.NewDecoder(r.Body)
	v := struct {
		Post   string `json:"post"`
		User   string `json:"user"`
		Upvote bool   `json:"upvote"`
	}{}
	if err := decoder.Decode(&v); err != nil {
		http.Error(w, "Failed to decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	vote(v.Post, v.User, v.Upvote)
	count := countVotes(v.Post)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(struct {
		Votes int `json:"votes"`
	}{count}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	posts = []Post{
		Post{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Color:     "ff0000",
			Text:      "Welcome To Dodel!",
			Timestamp: time.Now(),
			Image:     nil,
		},
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/")
		w.Write([]byte("hello\n"))
	})
	http.HandleFunc("/add", addPost)
	http.HandleFunc("/list", listPosts)
	http.HandleFunc("/show", showPost)
	http.HandleFunc("/vote", votePost)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
	}
}
