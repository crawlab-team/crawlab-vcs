package vcs

import "errors"

var (
	ErrInvalidActionsForBareRepo = errors.New("invalid actions for bare repo")
	ErrInvalidArgsLength         = errors.New("invalid arguments length")
	ErrUnsupportedType           = errors.New("unsupported type")
	ErrInvalidOptions            = errors.New("invalid options")
)
