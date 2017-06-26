package models

import "time"

// Vote stores information about a user's vote on a post.
type Vote struct {
	PostID    string
	UserID    string
	Upvote    bool
	Timestamp time.Time
}
