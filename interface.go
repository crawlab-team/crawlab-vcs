package vcs

type Client interface {
	Init(path string, args ...interface{}) (err error)
	Checkout(args ...interface{}) (err error)
	Commit(msg string, args ...interface{}) (err error)
	Pull(args ...interface{}) (err error)
	Push(args ...interface{}) (err error)
	Reset(args ...interface{}) (err error)
}
