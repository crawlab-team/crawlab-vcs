package vcs

import (
	"time"
)

type GitOptions struct {
	checkout []GitCheckoutOption
}

type GitLog struct {
	Msg         string    `json:"msg"`
	Branch      string    `json:"branch"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	Timestamp   time.Time `json:"timestamp"`
}

type GitFileStatus struct {
	Path     string          `json:"path"`
	Name     string          `json:"name"`
	IsDir    bool            `json:"is_dir"`
	Staging  string          `json:"staging"`
	Worktree string          `json:"worktree"`
	Extra    string          `json:"extra"`
	Children []GitFileStatus `json:"children"`
}
