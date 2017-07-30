package models

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"
)

// GetKarma computes the amount of karma a author has earned.
func GetKarma(c context.Context, author string) int {
	count, err := datastore.NewQuery("Post").Filter("Author =", author).Count(c)
	if err != nil {
		return 0
	}
	return count
}

func voteKey(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, "Vote", "", id, nil)
}

// Vote stores information about a user's vote on a post.
type Vote struct {
	Post   int64
	Author string
	Upvote bool
	Date   time.Time
}

// SubmitVote puts out a vote and updates the votes rank.
func SubmitVote(c context.Context, author string, id int64, upvote bool) (int64, error) {
	if _, err := GetPost(c, id); err != nil {
		return 0, fmt.Errorf("SubmitVote: could not find post: %v", err)
	}
	// verify input
	author = strings.TrimSpace(author)
	if len(author) < 1 {
		return 0, fmt.Errorf("SubmitVote: vote need author")
	}
	if voted, err := HasVotedOn(c, id, author); err != nil {
		return 0, fmt.Errorf("SubmitVote: failed to retrieve vote status: %v", err)
	} else if voted {
		return 0, fmt.Errorf("SubmitVote: user already voted on post")
	}
	vote := Vote{
		Post:   id,
		Author: author,
		Upvote: upvote,
		Date:   time.Now(),
	}
	baseKey := datastore.NewIncompleteKey(c, "Vote", nil)
	key, err := datastore.Put(c, baseKey, &vote)
	if err != nil {
		return 0, fmt.Errorf("SubmitVote: could not submit vote: %v", err)
	}
	if err := UpdateRank(c, id); err != nil {
		return 0, fmt.Errorf("SubmitVote: could not update rank: %v", err)
	}
	return key.IntID(), nil
}

// GetVote retrieves a vote from the database.
func GetVote(c context.Context, id int64) (Vote, error) {
	var vote Vote
	key := voteKey(c, id)
	if err := datastore.Get(c, key, &vote); err != nil {
		return vote, fmt.Errorf("GetVote: could not find vote: %v", err)
	}
	return vote, nil
}

// GetVoteBy retrieves a vote on a post by a user.
func GetVoteBy(c context.Context, id int64, author string) (Vote, error) {
	var votes []Vote
	if _, err := datastore.NewQuery("Vote").Filter("Author =", author).Filter("Post =", id).GetAll(c, &votes); err != nil {
		return Vote{}, fmt.Errorf("GetVoteBy: could not collect votes: %v", err)
	}
	if len(votes) != 1 {
		return Vote{}, fmt.Errorf("GetVoteBy: user has voted multiple times or never")
	}
	return votes[0], nil
}

// HasVotedOn retrieves if the user has submitted a vote on the given post.
func HasVotedOn(c context.Context, post int64, author string) (bool, error) {
	count, err := datastore.NewQuery("Vote").Filter("Author =", author).Filter("Post =", post).Count(c)
	if err != nil {
		return false, fmt.Errorf("HasVotedOn: could not collect votes: %v", err)
	}
	return count > 0, nil
}

// NumberOfVotes calculates the number of votes a post has received. This is a relative number.
func NumberOfVotes(c context.Context, id int64) (int, error) {
	var votes []Vote
	_, err := datastore.NewQuery("Vote").Filter("Post =", id).GetAll(c, &votes)
	if err != nil {
		return 0, fmt.Errorf("NumberOfVotes: could not collect votes: %v", err)
	}
	var sum int
	for _, vote := range votes {
		if vote.Upvote {
			sum++
		} else {
			sum--
		}
	}
	return sum, nil
}

// JSONVote is a JSON representation of a Vote.
type JSONVote struct {
	Author string `json:"user"`
	Post   int64  `json:"post"`
	Upvote bool   `json:"upvote"`
	Date   int64  `json:"time"`
}

func postKey(c context.Context, id int64) *datastore.Key {
	return datastore.NewKey(c, "Post", "", id, nil)
}

// TopPosts collects the top n posts with a minimum rank of x from the datastore.
func TopPosts(c context.Context, limit int, rank float64) ([]Post, []int64, error) {
	var posts []Post
	keys, err := datastore.NewQuery("Post").Filter("Parent =", 0).Order("Rank").Limit(limit).GetAll(c, &posts)
	if err != nil {
		return nil, nil, fmt.Errorf("TopPosts: could not collect posts: %v", err)
	}
	ids := make([]int64, len(keys))
	for i := range keys {
		ids[i] = keys[i].IntID()
	}
	return posts, ids, nil
}

// UpdateRank recomputes the rank of a post.
func UpdateRank(c context.Context, id int64) error {
	var post Post
	if err := datastore.Get(c, postKey(c, id), &post); err != nil {
		return fmt.Errorf("UpdateRank: could not find post: %v", err)
	}
	num, err := NumberOfVotes(c, id)
	if err != nil {
		return fmt.Errorf("UpdateRank: could not count votes: %v", err)
	}
	post.Rank = float64(num)
	if _, err := datastore.Put(c, postKey(c, id), &post); err != nil {
		return fmt.Errorf("UpdateRank: could not save changes: %v", err)
	}
	return nil
}

// SubmitPost stores a post in the datastore.
func SubmitPost(c context.Context, author, text, color string, parent int64) (int64, error) {
	if parent != 0 {
		if _, err := GetPost(c, parent); err != nil {
			return 0, fmt.Errorf("SubmitPost: could not find parent: %v", err)
		}
	}
	// verify input
	author = strings.TrimSpace(author)
	text = strings.TrimSpace(text)
	if len(author) < 1 || len(text) < 1 {
		return 0, fmt.Errorf("SubmitPost: Can not submit empty post")
	}
	post := Post{
		Author: author,
		Parent: parent,
		Text:   text,
		Color:  color,
		Date:   time.Now(),
		Rank:   0,
	}
	baseKey := datastore.NewIncompleteKey(c, "Post", nil)
	key, err := datastore.Put(c, baseKey, &post)
	if err != nil {
		return 0, fmt.Errorf("SubmitPost: could not submit post: %v", err)
	}
	return key.IntID(), nil
}

// NumberOfComments retrieves the number of comments a post has received.
func NumberOfComments(c context.Context, id int64) (int, error) {
	count, err := datastore.NewQuery("Post").Filter("Parent =", id).Count(c)
	if err != nil {
		return 0, fmt.Errorf("NumberOfComments: could not collect posts: %v", err)
	}
	return count, nil
}

// GetPost retrieves a post from the datastore.
func GetPost(c context.Context, id int64) (Post, error) {
	var post Post
	if err := datastore.Get(c, postKey(c, id), &post); err != nil {
		return post, fmt.Errorf("GetPost: could not find post: %v", err)
	}
	return post, nil
}

// GetComments retrieves all comments on the specified post ordered by rank.
func GetComments(c context.Context, id int64) ([]Post, []int64, error) {
	var comments []Post
	keys, err := datastore.NewQuery("Post").Filter("Parent =", id).Order("Date").GetAll(c, &comments)
	if err != nil {
		return nil, nil, fmt.Errorf("GetComments: could not collect posts: %v", err)
	}
	ids := make([]int64, len(keys))
	for i, k := range keys {
		ids[i] = k.IntID()
	}
	return comments, ids, nil
}

// ToJSONComments converts a slice of comments to a JSON serializable slice.
func ToJSONComments(c context.Context, comments []Post, ids []int64) ([]JSONPost, error) {
	if len(comments) != len(ids) {
		return nil, fmt.Errorf("ToJSONComments: array size does not match")
	}

	var err error
	jsonComments := make([]JSONPost, len(comments))
	for i := range comments {
		jsonComments[i], err = ToJSONPost(c, ids[i], comments[i])
		if err != nil {
			return nil, fmt.Errorf("ToJSONComments: %v", err)
		}
	}
	return jsonComments, nil
}

// ToJSONPost converts the post to a JSON serializable representation.
func ToJSONPost(c context.Context, id int64, post Post) (JSONPost, error) {
	numVotes, err := NumberOfVotes(c, id)
	if err != nil {
		return JSONPost{}, fmt.Errorf("GetJSONPost: %v", err)
	}
	numComments, err := NumberOfComments(c, id)
	if err != nil {
		return JSONPost{}, fmt.Errorf("GetJSONPost: %v", err)
	}

	return JSONPost{
		ID:       id,
		Parent:   post.Parent,
		Date:     post.Date.Unix(),
		Author:   post.Author,
		Text:     post.Text,
		Color:    post.Color,
		Votes:    numVotes,
		Comments: numComments,
	}, nil
}

// Post stores information about a user's post like ID, userID and topicID.
type Post struct {
	Author string
	Parent int64
	Text   string
	Color  string
	Date   time.Time
	Rank   float64
}

// JSONPost is a JSON represenation of a Post.
type JSONPost struct {
	ID       int64  `json:"id"`
	Parent   int64  `json:"topic"`
	Date     int64  `json:"timestamp"`
	Author   string `json:"user"`
	Text     string `json:"text"`
	Votes    int    `json:"votes"`
	Color    string `json:"color"`
	Comments int    `json:"comments"`
}
