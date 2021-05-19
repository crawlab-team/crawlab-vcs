package vcs

import (
	"github.com/go-git/go-git/v5"
	"os"
)

func CreateBareGitRepo(path string) (err error) {
	// validate options
	if path == "" {
		return ErrInvalidRepoPath
	}

	// validate if exists
	if IsGitRepoExists(path) {
		return ErrRepoAlreadyExists
	}

	// create directory if not exists
	_, err = os.Stat(path)
	if err != nil {
		if err := os.MkdirAll(path, os.FileMode(0766)); err != nil {
			return err
		}
		err = nil
	}

	// init
	if _, err := git.PlainInit(path, true); err != nil {
		return err
	}

	return nil
}

func CloneGitRepo(path, url string, opts ...GitCloneOption) (c *GitClient, err error) {
	// url
	opts = append(opts, WithURL(url))

	// apply options
	o := &git.CloneOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// clone
	if _, err := git.PlainClone(path, false, o); err != nil {
		return nil, err
	}

	return NewGitClient(WithPath(path))
}

func IsGitRepoExists(path string) (ok bool) {
	if _, err := git.PlainOpen(path); err != nil {
		return false
	}
	return true
}
