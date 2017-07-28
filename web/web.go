package web

import (
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strings"

	"github.com/lnsp/zwig/models"
	"github.com/lnsp/zwig/utils"
	"github.com/pborman/uuid"
)

// the duration of the auth cookie
const (
	authCookieDuration = 608400
)

// collection of template file names
const (
	baseTemplateFile = "static/templates/base.html"
	showTemplateFile = "static/templates/show.html"
	listTemplateFile = "static/templates/list.html"
)

// collection of available colors
var (
	colors = [4]string{"blue", "red", "orange", "green"}
)

type authHandleFunc func(http.ResponseWriter, *http.Request, bool, string)

// Handler presents a Web UI to interact with posts.
type Handler struct {
	mux                *http.ServeMux
	database           models.Database
	debug              bool
	listTmpl, showTmpl *template.Template
}

// New initializes a new web handler.
func New(database models.Database, debug bool) *Handler {
	mux := http.NewServeMux()
	web := &Handler{mux, database, debug, nil, nil}
	// load templates
	web.listTmpl = template.Must(template.ParseFiles(baseTemplateFile, listTemplateFile))
	web.showTmpl = template.Must(template.ParseFiles(baseTemplateFile, showTemplateFile))
	// add static resource routes
	mux.Handle("/static/css/", http.StripPrefix("/static/css/", http.FileServer(http.Dir("static/css"))))
	mux.Handle("/static/js/", http.StripPrefix("/static/js/", http.FileServer(http.Dir("static/js"))))
	// add dynamic routes
	mux.Handle("/", web.auth(web.list, false))
	mux.Handle("/comments", web.auth(web.comments, false))
	mux.Handle("/post", web.auth(web.post, true))
	mux.Handle("/vote", web.auth(web.vote, true))
	return web
}

// ServeHTTP serves HTTP requests.
func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler.debug {
		log.Printf("web: %s\n", r.URL)
	}
	handler.mux.ServeHTTP(w, r)
}

// template-internal post representation
type postItem struct {
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

func (handler *Handler) list(w http.ResponseWriter, r *http.Request, auth bool, user string) {
	posts := handler.database.List(models.DefaultListCount, models.DefaultMaxAge, models.DefaultMinRank)
	items := make([]postItem, len(posts))
	for i, post := range posts {
		items[i] = handler.toPostItem(post, user)
	}
	if err := handler.listTmpl.Execute(w, struct {
		Karma     int
		NextColor string
		Posts     []postItem
	}{
		Karma:     handler.database.Karma(user),
		NextColor: colors[rand.Intn(len(colors))],
		Posts:     items,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}

func (handler *Handler) comments(w http.ResponseWriter, r *http.Request, auth bool, user string) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if uuid.Parse(id) == nil {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	comments := handler.database.Comments(id)
	items := make([]postItem, len(comments))
	for i, comment := range comments {
		items[i] = handler.toPostItem(comment, user)
	}
	main := handler.toPostItem(handler.database.Get(id), user)
	if err := handler.showTmpl.Execute(w, struct {
		Karma     int
		NextColor string
		Main      postItem
		Comments  []postItem
	}{
		Karma:     handler.database.Karma(user),
		NextColor: colors[rand.Intn(len(colors))],
		Main:      main,
		Comments:  items,
	}); err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
	}
}

func (handler *Handler) post(w http.ResponseWriter, r *http.Request, auth bool, user string) {
	if !auth {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	text := r.FormValue("text")
	color := r.FormValue("color")
	topic := r.FormValue("topic")
	keep := r.FormValue("keep")

	if handler.debug {
		log.Printf("web.post: user=%s color=%s topic=%s keep=%s\n", user, color, topic, keep)
	}

	redirectURL := "/"
	if keep != "" {
		redirectURL = "/comments?id=" + topic
	}
	if strings.TrimSpace(text) == "" {
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return
	}
	handler.database.Post(user, text, color, topic, nil)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (handler *Handler) vote(w http.ResponseWriter, r *http.Request, auth bool, user string) {
	if !auth {
		if handler.debug {
			log.Println("web.vote: user not authorized")
		}
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	upvote := r.FormValue("upvote")
	downvote := r.FormValue("downvote")
	post := r.FormValue("post")
	keep := r.FormValue("keep")
	topic := r.FormValue("topic")

	if handler.debug {
		log.Printf("web.vote: post=%s keep=%s topic=%s upvote=%s downvote%s\n", post, keep, topic, upvote, downvote)
	}

	redirectURL := "/"
	if keep != "" {
		redirectURL = "/comments?id=" + topic
	}
	if upvote != "" {
		handler.database.Vote(user, post, true)
	} else if downvote != "" {
		handler.database.Vote(user, post, false)
	}
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

type authMiddleware struct {
	handler  authHandleFunc
	required bool
}

func (auth authMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("user_uid")
	if err != nil && !auth.required {
		user := uuid.New()
		http.SetCookie(w, &http.Cookie{
			Name:   "user_uid",
			Value:  user,
			MaxAge: authCookieDuration,
		})
		//log.Printf("web.auth: new user granted access to %s\n", r.URL)
		auth.handler(w, r, true, user)
	} else if auth.required && err != nil {
		//log.Printf("web.auth: user not authorized to access %s\n", r.URL)
		auth.handler(w, r, false, "")
	} else {
		//log.Printf("web.auth: existing user allowed to pass to %s\n", r.URL)
		auth.handler(w, r, true, cookie.Value)
	}
}

func (handler *Handler) auth(f authHandleFunc, require bool) *authMiddleware {
	return &authMiddleware{
		handler:  f,
		required: require,
	}
}

func (handler *Handler) toPostItem(post models.Post, user string) postItem {
	state := handler.database.VoteState(post.ID, user)
	return postItem{
		Post:         post.ID,
		User:         post.UserID,
		Text:         post.Text,
		Votes:        handler.database.CountVotes(post.ID),
		Color:        post.Color,
		Topic:        post.TopicID,
		OwnPost:      post.UserID == user,
		HasUpvoted:   state == models.VoteUp,
		HasDownvoted: state == models.VoteDown,
		SincePost:    utils.HumanTimeFormat(post.Timestamp),
	}
}
