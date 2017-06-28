package models

import (
	"encoding/json"
	"fmt"
	"image"
	"log"
	"os"
	"sort"
	"time"

	"github.com/pborman/uuid"
)

// VoteState stores the stance a user has taken on a specific post.
type VoteState string

const (
	// VoteNone means no action taken.
	VoteNone VoteState = "none"
	// VoteUp means upvoted.
	VoteUp VoteState = "upvote"
	// VoteDown means downvoted.
	VoteDown VoteState = "downvote"
)

// Collection of defaults needed for database.List calls.
const (
	DefaultListCount = 30
	DefaultMinRank   = -5.0
	DefaultMaxAge    = time.Hour * 24
)

// Database stores and retrieves user data.
type Database interface {
	Get(id string) Post
	Post(user, text, color, topic string, img image.Image) string
	Vote(user, post string, upvote bool)
	VoteState(id, user string) VoteState
	Votes(post string) []Vote
	CountVotes(id string) int
	CountComments(id string) int
	Comments(id string) []Post
	Karma(user string) int
	List(count int, maxAge time.Duration, minRank float64) []Post
	ToJSONVote(vote Vote) JSONVote
	ToJSONVotes(votes []Vote) []JSONVote
	ToJSONPost(post Post) JSONPost
	ToJSONPosts(posts []Post) []JSONPost
}

// JSONDatabase is a database reading and writing to a JSON file.
type JSONDatabase struct {
	posts    []Post
	votes    []Vote
	name     string
	override bool
	debug    bool
}

type dbFileStruct struct {
	Posts []JSONPost `json:"posts"`
	Votes []JSONVote `json:"votes"`
}

// NewJSONDatabase initializes a new JSON database.
func NewJSONDatabase(name string, override, debug bool) *JSONDatabase {
	return &JSONDatabase{
		name:     name,
		override: override,
		debug:    debug,
	}
}

// JSONPost is a JSON represenation of a Post.
type JSONPost struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	User      string `json:"user"`
	Text      string `json:"text"`
	Votes     int    `json:"votes"`
	Color     string `json:"color"`
	Timestamp int64  `json:"timestamp"`
	Comments  int    `json:"comments"`
}

// Convert converts a JSON post into a normal database compatible struct.
func (post JSONPost) Convert() Post {
	return Post{
		ID:        post.ID,
		UserID:    post.User,
		TopicID:   post.Topic,
		Text:      post.Text,
		Color:     post.Color,
		Timestamp: time.Unix(post.Timestamp, 0),
	}
}

// ToJSONPost converts a normal post into a JSON compatible struct.
func (db *JSONDatabase) ToJSONPost(post Post) JSONPost {
	return JSONPost{
		ID:        post.ID,
		User:      post.UserID,
		Topic:     post.TopicID,
		Color:     post.Color,
		Text:      post.Text,
		Timestamp: post.Timestamp.Unix(),
		Votes:     db.CountVotes(post.ID),
		Comments:  db.CountComments(post.ID),
	}
}

// ToJSONPosts converts a slice of normal posts into a JSON compatible slice.
func (db *JSONDatabase) ToJSONPosts(posts []Post) []JSONPost {
	json := make([]JSONPost, len(posts))
	for i, post := range posts {
		json[i] = db.ToJSONPost(post)
	}
	return json
}

// JSONVote is a JSON representation of a Vote.
type JSONVote struct {
	User      string `json:"user"`
	Post      string `json:"post"`
	Upvote    bool   `json:"upvote"`
	Timestamp int64  `json:"time"`
}

// Convert converts a JSON vote into a normal database compatible struct.
func (vote JSONVote) Convert() Vote {
	return Vote{
		PostID:    vote.Post,
		UserID:    vote.User,
		Upvote:    vote.Upvote,
		Timestamp: time.Unix(vote.Timestamp, 0),
	}
}

// ToJSONVote converts a normal vote into a JSON compatible struct.
func (db *JSONDatabase) ToJSONVote(vote Vote) JSONVote {
	return JSONVote{
		User:      vote.UserID,
		Post:      vote.PostID,
		Upvote:    vote.Upvote,
		Timestamp: vote.Timestamp.Unix(),
	}
}

// ToJSONVotes converts a slice of normal votes into a JSON compatible slice.
func (db *JSONDatabase) ToJSONVotes(votes []Vote) []JSONVote {
	json := make([]JSONVote, len(votes))
	for i, vote := range votes {
		json[i] = db.ToJSONVote(vote)
	}
	return json
}

// Save database to JSON file.
func (db *JSONDatabase) Save() error {
	file, err := os.Create(db.name)
	if err != nil {
		return fmt.Errorf("db.save: %v", err)
	}
	defer file.Close()

	jsonPosts := make([]JSONPost, len(db.posts))
	jsonVotes := make([]JSONVote, len(db.votes))

	for i, post := range db.posts {
		jsonPosts[i] = db.ToJSONPost(post)
	}
	for i, vote := range db.votes {
		jsonVotes[i] = db.ToJSONVote(vote)
	}

	enc := json.NewEncoder(file)
	if err := enc.Encode(dbFileStruct{
		Posts: jsonPosts,
		Votes: jsonVotes,
	}); err != nil {
		return fmt.Errorf("db.load: %v", err)
	}

	return nil
}

// Load fetches JSON data from the filesystem and decodes it.
func (db *JSONDatabase) Load() error {
	file, err := os.Open(db.name)
	if err != nil && db.override {
		db.posts = make([]Post, 0)
		db.votes = make([]Vote, 0)
		if db.debug {
			log.Printf("db.load: %v\n", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("db.load: %v", err)
	}
	defer file.Close()
	dbFile := dbFileStruct{}
	dec := json.NewDecoder(file)
	if err := dec.Decode(&dbFile); err != nil {
		return fmt.Errorf("db.load: %v", err)
	}
	db.posts = make([]Post, len(dbFile.Posts))
	db.votes = make([]Vote, len(dbFile.Votes))
	for i, post := range dbFile.Posts {
		db.posts[i] = post.Convert()
	}
	for i, vote := range dbFile.Votes {
		db.votes[i] = vote.Convert()
	}
	if db.debug {
		log.Printf("db.load: loaded %d posts, %d votes\n", len(db.posts), len(db.votes))
	}
	db.recomputeAll()
	if db.debug {
		log.Println("db.load: recomputed post ranks")
	}
	return nil
}

// Recompute all post ranks.
func (db *JSONDatabase) recomputeAll() {
	for _, post := range db.posts {
		db.computeRank(post.ID)
	}
}

// Computes a single posts rank value.
func (db *JSONDatabase) computeRank(id string) {
	post := db.Get(id)
	votes := float64(db.CountVotes(id))
	elapsed := time.Since(post.Timestamp).Minutes()
	rank := elapsed*elapsed - votes*votes*votes
	for _, post := range db.posts {
		if post.ID != id {
			continue
		}
		post.Rank = rank
	}
}

// VoteState returns the stance a user has taken on a post.
func (db *JSONDatabase) VoteState(id, user string) VoteState {
	state := VoteNone
	for _, vote := range db.votes {
		if vote.PostID == id && vote.UserID == user {
			if vote.Upvote {
				state = VoteUp
			} else {
				state = VoteDown
			}
		}
	}
	return state
}

// CountComments retrieves the amount of comments a post has received.
func (db *JSONDatabase) CountComments(id string) int {
	count := 0
	for _, post := range db.posts {
		if post.TopicID == id {
			count++
		}
	}
	return count
}

// CountVotes retrieves the amount of upvotes a post has received.
func (db *JSONDatabase) CountVotes(id string) int {
	voted := make(map[string]int)
	for _, vote := range db.votes {
		if vote.PostID != id {
			continue
		}
		if vote.Upvote {
			voted[vote.UserID] = 1
		} else {
			voted[vote.UserID] = -1
		}
	}
	count := 0
	for _, decision := range voted {
		count += decision
	}
	return count
}

// Votes retrieves the instances of votes a post has received.
func (db *JSONDatabase) Votes(post string) []Vote {
	votes := make([]Vote, 0)
	for _, vote := range db.votes {
		if vote.PostID != post {
			continue
		}
		votes = append(votes, vote)
	}
	return votes
}

// Vote on a user's post.
func (db *JSONDatabase) Vote(user, post string, upvote bool) {
	db.votes = append(db.votes, Vote{
		UserID:    user,
		PostID:    post,
		Upvote:    upvote,
		Timestamp: time.Now(),
	})
	db.computeRank(post)
}

// Karma retrieves the user's karma score.
func (db *JSONDatabase) Karma(user string) int {
	count := 0
	for _, post := range db.posts {
		if post.UserID != user {
			continue
		}
		count += db.CountVotes(post.ID)
	}
	return count
}

// Post adds a new entry to the dodelnet.
func (db *JSONDatabase) Post(user, text, color, topic string, img image.Image) string {
	id := uuid.New()
	post := Post{
		ID:        id,
		UserID:    user,
		TopicID:   topic,
		Text:      text,
		Color:     color,
		Image:     img,
		Timestamp: time.Now(),
	}
	db.posts = append(db.posts, post)
	return id
}

// List returns a sorted slice of the latest and most popular posts.
// maxAge should be in minutes.
func (db *JSONDatabase) List(count int, maxAge time.Duration, minRank float64) []Post {
	posts := make([]Post, 0)

	for _, post := range db.posts {
		// Post is too old
		if time.Since(post.Timestamp) > maxAge {
			continue
		}
		// Post has too low votes
		if post.Rank < minRank {
			continue
		}
		// Post is a comment
		if post.TopicID != "" {
			continue
		}
		posts = append(posts, post)
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Rank < posts[j].Rank
	})

	if len(posts) < count {
		return posts
	}

	return posts[:count]
}

// Get returns a user's post identified by its id.
func (db *JSONDatabase) Get(id string) Post {
	for _, post := range db.posts {
		if post.ID == id {
			return post
		}
	}
	return Post{}
}

// Comments returns a sorted slice of comments on the specified topic post.
func (db *JSONDatabase) Comments(id string) []Post {
	comments := make([]Post, 0)
	for _, post := range db.posts {
		if post.TopicID != id {
			continue
		}
		comments = append(comments, post)
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].Timestamp.Before(comments[j].Timestamp)
	})
	return comments
}
