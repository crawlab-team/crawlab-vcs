package vcs

import "time"

type GitOptions struct {
	checkout []GitCheckoutOption
}

type GitLog struct {
	Msg         string
	Branch      string
	AuthorName  string
	AuthorEmail string
	Timestamp   time.Time
}
