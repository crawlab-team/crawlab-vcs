package vcs

import "errors"

var (
	ErrInvalidArgsLength = errors.New("invalid arguments length")
	ErrUnsupportedType   = errors.New("unsupported type")
	ErrInvalidOptions    = errors.New("invalid options")
	ErrRepoAlreadyExists = errors.New("repo already exists")
	ErrInvalidRepoPath   = errors.New("invalid repo path")
)
