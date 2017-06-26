package models

import (
	"image"
	"time"
)

// Post stores information about a user's post like ID, userID and topicID.
type Post struct {
	ID        string
	UserID    string
	TopicID   string
	Text      string
	Color     string
	Image     image.Image
	Timestamp time.Time
	Rank      float64
}
