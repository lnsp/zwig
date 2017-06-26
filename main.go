package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"image"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/pborman/uuid"
)

var (
	hostport = flag.String("host", ":8080", "Set host port")
)

type Post struct {
	ID        string
	UserID    string
	TopicID   string
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
	colors = []string{"blue", "red", "orange", "green"}
	posts  = []Post{}
	votes  = []Vote{}
)

type postJSON struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	User      string `json:"user"`
	Text      string `json:"text"`
	Votes     int    `json:"votes"`
	Color     string `json:"color"`
	Timestamp int64  `json:"timestamp"`
	Comments  int    `json:"comments"`
}

type voteJSON struct {
	User      string `json:"user"`
	Post      string `json:"post"`
	Upvote    bool   `json:"upvote"`
	Timestamp int64  `json:"time"`
}

type Database struct {
	Posts []postJSON `json:"posts"`
	Votes []voteJSON `json:"votes"`
}

func save(name string) {
	file, err := os.Create(name)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()
	db := Database{
		make([]postJSON, 0),
		make([]voteJSON, 0),
	}
	for _, p := range posts {
		db.Posts = append(db.Posts, postJSON{
			ID:        p.ID,
			User:      p.UserID,
			Topic:     p.TopicID,
			Color:     p.Color,
			Text:      p.Text,
			Timestamp: p.Timestamp.Unix(),
		})
	}
	for _, v := range votes {
		db.Votes = append(db.Votes, voteJSON{
			User:      v.UserID,
			Post:      v.PostID,
			Upvote:    v.Upvote,
			Timestamp: v.Timestamp.Unix(),
		})
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(db); err != nil {
		log.Println(err)
		return
	}
	log.Println("saved")
}

func restore(name string) {
	file, err := os.Open(name)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()
	db := Database{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&db); err != nil {
		log.Println(err)
		return
	}
	for _, p := range db.Posts {
		posts = append(posts, Post{
			ID:        p.ID,
			UserID:    p.User,
			TopicID:   p.Topic,
			Text:      p.Text,
			Color:     p.Color,
			Timestamp: time.Unix(p.Timestamp, 0),
		})
	}
	for _, v := range db.Votes {
		votes = append(votes, Vote{
			PostID:    v.Post,
			UserID:    v.User,
			Upvote:    v.Upvote,
			Timestamp: time.Unix(v.Timestamp, 0),
		})
	}
	log.Println("restored")
}

func vote(user, post string, upvote bool) {
	votes = append(votes, Vote{
		PostID:    post,
		UserID:    user,
		Upvote:    upvote,
		Timestamp: time.Now(),
	})
}

func karma(user string) int {
	count := 0
	for _, p := range posts {
		if p.UserID == user {
			if votes := countVotes(p.ID); votes > 0 {
				count += votes
			}
		}
	}
	return count
}

func post(user, text, color, topic string, img image.Image) string {
	id := uuid.New()
	instance := Post{
		ID:        id,
		UserID:    user,
		TopicID:   topic,
		Text:      text,
		Color:     color,
		Image:     img,
		Timestamp: time.Now(),
	}
	posts = append(posts, instance)
	log.Println("adding post", instance)
	return id
}

func list() []Post {
	results := []Post{}
	for _, p := range posts {
		if time.Since(p.Timestamp) > time.Hour*24 || p.TopicID != "" {
			continue
		}
		results = append(results, p)
	}

	rank := func(p Post) float64 {
		votes := countVotes(p.ID)
		ts := time.Since(p.Timestamp).Minutes()*time.Since(p.Timestamp).Minutes() - float64(votes*votes*votes)
		return ts
	}

	sort.Slice(results, func(i, j int) bool {
		return rank(results[i]) < rank(results[j])
	})

	return results
}

func voteState(id, user string) string {
	state := "none"
	for _, v := range votes {
		if v.PostID == id && v.UserID == user {
			if v.Upvote {
				state = "upvote"
			} else {
				state = "downvote"
			}
		}
	}
	return state
}

func countComments(id string) int {
	count := 0
	for _, p := range posts {
		if p.TopicID == id {
			count++
		}
	}
	return count
}

func countVotes(id string) int {
	voted := map[string]int{}
	for _, v := range votes {
		if v.PostID != id {
			continue
		}
		if v.Upvote {
			voted[v.UserID] = 1
		} else {
			voted[v.UserID] = -1
		}
	}
	count := 0
	for _, k := range voted {
		count += k
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
		Topic string `json:"topic"`
	}{}
	if err := decoder.Decode(&add); err != nil {
		http.Error(w, "Failed to parse JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	id := post(add.User, add.Text, add.Color, add.Topic, nil)

	buf, err := json.Marshal(struct {
		ID string `json:"id"`
	}{id})
	if err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf)
}

func toJSONPosts(target []Post) []postJSON {
	resultsJSON := []postJSON{}
	for _, p := range target {
		post := postJSON{
			ID:        p.ID,
			User:      p.UserID,
			Color:     p.Color,
			Text:      p.Text,
			Votes:     countVotes(p.ID),
			Timestamp: p.Timestamp.Unix(),
			Comments:  countComments(p.ID),
		}
		resultsJSON = append(resultsJSON, post)
	}
	return resultsJSON
}

func listPosts(w http.ResponseWriter, r *http.Request) {
	log.Println("/list")

	results := list()
	resultsJSON := toJSONPosts(results)

	if buf, err := json.Marshal(resultsJSON); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(buf)
	}
}

func comments(id string) []Post {
	result := []Post{}
	for _, p := range posts {
		if p.TopicID == id {
			result = append(result, p)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
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
	postComments := comments(selected.ID)
	log.Println(postComments)
	if err := encoder.Encode(struct {
		ID        string     `json:"id"`
		User      string     `json:"user"`
		Text      string     `json:"text"`
		Votes     int        `json:"votes"`
		Timestamp int64      `json:"timestamp"`
		Color     string     `json:"color"`
		Comments  []postJSON `json:"comments"`
	}{
		Color:     selected.Color,
		ID:        selected.ID,
		User:      selected.UserID,
		Text:      selected.Text,
		Votes:     countVotes(selected.ID),
		Timestamp: selected.Timestamp.Unix(),
		Comments:  toJSONPosts(postComments),
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

	vote(v.User, v.Post, v.Upvote)
	count := countVotes(v.Post)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(struct {
		Votes int `json:"votes"`
	}{count}); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

func static() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/css/", http.StripPrefix("/static/css/", http.FileServer(http.Dir("static/css"))))
	mux.Handle("/static/js/", http.StripPrefix("/static/js/", http.FileServer(http.Dir("static/js"))))
	return mux
}

func api() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/")
		w.Write([]byte("hello\n"))
	})
	mux.HandleFunc("/add", addPost)
	mux.HandleFunc("/list", listPosts)
	mux.HandleFunc("/show", showPost)
	mux.HandleFunc("/vote", votePost)
	return mux
}

func webhandler() http.Handler {
	listTemplate := template.Must(template.ParseFiles("static/templates/base.html", "static/templates/list.html"))
	showTemplate := template.Must(template.ParseFiles("static/templates/base.html", "static/templates/show.html"))
	mux := http.NewServeMux()
	type webPost struct {
		User         string `json:"user"`
		Text         string `json:"text"`
		Topic        string `json:"topic"`
		Votes        int    `json:"votes"`
		Color        string `json:"color"`
		Post         string `json:"post"`
		OwnPost      bool   `json:"own"`
		HasUpvoted   bool   `json:"upvoted"`
		HasDownvoted bool   `json:"downvoted"`
		SincePost    string `json:"since"`
	}
	toWebPost := func(p Post, user string) webPost {
		state := voteState(p.ID, user)
		return webPost{
			User:         p.UserID,
			Text:         p.Text,
			Votes:        countVotes(p.ID),
			Color:        p.Color,
			Topic:        p.TopicID,
			Post:         p.ID,
			OwnPost:      p.UserID == user,
			HasUpvoted:   state == "upvote",
			HasDownvoted: state == "downvote",
			SincePost:    timeSinceHuman(p.Timestamp),
		}
	}
	postWithID := func(id string) Post {
		for _, p := range posts {
			if p.ID == id {
				return p
			}
		}
		return Post{}
	}
	uidMiddleware := func(w http.ResponseWriter, r *http.Request, force bool) string {
		cookie, err := r.Cookie("user_uid")
		if err != nil && !force {
			user := uuid.New()
			http.SetCookie(w, &http.Cookie{
				Name:   "user_uid",
				Value:  user,
				MaxAge: 604800,
			})
			return user
		} else if force && err != nil {
			return ""
		}
		return cookie.Value
	}
	mux.HandleFunc("/web/", func(w http.ResponseWriter, r *http.Request) {
		user := uidMiddleware(w, r, false)
		webposts := []webPost{}
		for _, p := range list() {
			webposts = append(webposts, toWebPost(p, user))
		}
		if err := listTemplate.Execute(w, struct {
			Karma     int
			NextColor string
			Posts     []webPost
		}{karma(user), colors[rand.Intn(len(colors))], webposts}); err != nil {
			http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/web/comments", func(w http.ResponseWriter, r *http.Request) {
		user := uidMiddleware(w, r, false)
		id := r.URL.Query().Get("id")

		log.Println("/comments", id, user)
		webposts := []webPost{}
		for _, p := range comments(id) {
			webposts = append(webposts, toWebPost(p, user))
		}
		main := toWebPost(postWithID(id), user)
		if err := showTemplate.Execute(w, struct {
			Karma     int
			NextColor string
			Main      webPost
			Comments  []webPost
		}{karma(user), colors[rand.Intn(len(colors))], main, webposts}); err != nil {
			http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/web/post", func(w http.ResponseWriter, r *http.Request) {
		user := uidMiddleware(w, r, true)
		if user == "" {
			http.Redirect(w, r, "/web/", http.StatusTemporaryRedirect)
			return
		}
		text := r.FormValue("text")
		color := r.FormValue("color")
		topic := r.FormValue("topic")
		keep := r.FormValue("keep")

		redirectURL := "/web/"
		if keep != "" {
			redirectURL = "/web/comments?id=" + topic
		}

		if strings.TrimSpace(text) == "" {
			http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			return
		}

		log.Println("/post", user, text)
		post(user, text, color, topic, nil)

		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/web/vote", func(w http.ResponseWriter, r *http.Request) {
		user := uidMiddleware(w, r, true)
		if user == "" {
			http.Redirect(w, r, "/web/", http.StatusTemporaryRedirect)
			return
		}
		upvote := r.FormValue("upvote")
		downvote := r.FormValue("downvote")
		post := r.FormValue("post")
		keep := r.FormValue("keep")
		topic := r.FormValue("topic")

		log.Println("/vote", user, upvote, downvote, post)

		redirectURL := "/web"
		if keep != "" {
			redirectURL = "/web/comments?id=" + topic
		}

		if upvote != "" {
			vote(user, post, true)
		} else if downvote != "" {
			vote(user, post, false)
		}

		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	})

	return mux
}

func timeSinceHuman(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 2 {
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	} else if duration.Hours() >= 1 {
		return "a hour ago"
	} else if duration.Minutes() >= 2 {
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	} else if duration.Minutes() >= 1 {
		return "a minute ago"
	} else {
		return "some seconds ago"
	}
}

func main() {
	flag.Parse()
	signals := make(chan os.Signal, 1)
	go func() {
		<-signals
		save("backup.json")
		os.Exit(1)
	}()
	signal.Notify(signals, os.Interrupt)
	restore("backup.json")
	http.Handle("/", api())
	http.Handle("/static/", static())
	http.Handle("/web/", webhandler())
	if err := http.ListenAndServe(*hostport, nil); err != nil {
		fmt.Println(err)
	}
}
