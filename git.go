package vcs

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"
)

type GitClient struct {
	r    *git.Repository
	opts *GitOptions
}

type GitOptions struct {
	Path           string
	RemoteUrl      string
	IsBare         bool
	IsMem          bool
	AuthType       GitAuthType
	Username       string
	Password       string
	PrivateKeyPath string
}

type GitLog struct {
	Msg         string
	Branch      string
	AuthorName  string
	AuthorEmail string
	Timestamp   time.Time
}

var DefaultGitOptions = GitOptions{
	Path:           "",
	RemoteUrl:      "",
	IsBare:         false,
	IsMem:          false,
	AuthType:       GitAuthTypeNone,
	Username:       "",
	Password:       "",
	PrivateKeyPath: getDefaultPublicKeyPath(),
}

var GitMemStorages = sync.Map{}
var GitMemFileSystem = sync.Map{}

func NewGitClient(options *GitOptions) (c *GitClient, err error) {
	if options == nil {
		options = &DefaultGitOptions
	}
	if options.PrivateKeyPath == "" {
		options.PrivateKeyPath = getDefaultPublicKeyPath()
	}
	c = &GitClient{
		opts: options,
	}
	if err := c.Init(options.IsBare); err != nil {
		return c, err
	}
	return
}

func getBranchAndHashAndIsCreateFromArgs(args ...interface{}) (branch plumbing.ReferenceName, hash plumbing.Hash, err error) {
	if len(args) < 2 {
		return branch, hash, ErrInvalidArgsLength
	}
	if args[0] != nil {
		branch = plumbing.NewBranchReferenceName(args[0].(string))
	}
	if args[1] != nil {
		hash = plumbing.NewHash(args[1].(string))
	}
	return
}

func getRemoteNameFromArgs(args ...interface{}) (remoteName string, err error) {
	if len(args) < 1 {
		return remoteName, ErrInvalidArgsLength
	}
	if args[0] == nil {
		remoteName = GitRemoteNameOrigin
	} else {
		remoteName = args[0].(string)
	}
	return
}

func getResetModeFromArgs(args ...interface{}) (mode git.ResetMode, err error) {
	if len(args) < 1 {
		return mode, ErrInvalidArgsLength
	}
	if args[0] != nil {
		mode, err = getResetMode(args[0])
		if err != nil {
			return mode, err
		}
	}
	return
}

func getResetMode(mode interface{}) (res git.ResetMode, err error) {
	switch mode.(type) {
	case int8:
		return git.ResetMode(int8(0)), nil
	case git.ResetMode:
		return mode.(git.ResetMode), err
	}
	return git.MixedReset, ErrUnsupportedType
}

func getGitAuth(authType GitAuthType, username, password, privateKeyPath string) (auth transport.AuthMethod, err error) {
	switch authType {
	case GitAuthTypeNone:
		auth = nil
	case GitAuthTypeHTTP:
		auth = &http.BasicAuth{
			Username: username,
			Password: password,
		}
	case GitAuthTypeSSH:
		privateKeyData, err := ioutil.ReadFile(privateKeyPath)
		if err != nil {
			return auth, err
		}
		var signer ssh.Signer
		if password != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(privateKeyData, []byte(password))
		} else {
			signer, err = ssh.ParsePrivateKey(privateKeyData)
		}
		if err != nil {
			return nil, err
		}
		auth = &gitssh.PublicKeys{
			User:   "git",
			Signer: signer,
			HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		}
	default:
		return auth, ErrUnsupportedType
	}
	return
}

func getDefaultPublicKeyPath() (path string) {
	u, err := user.Current()
	if err != nil {
		return path
	}
	path = filepath.Join(u.HomeDir, ".ssh", "id_rsa")
	return
}

func (c *GitClient) Init(args ...interface{}) (err error) {
	initType := c.GetInitType()
	switch initType {
	case GitInitTypeFs:
		err = c.InitFs()
	case GitInitTypeMem:
		err = c.InitMem()
	}
	if err != nil {
		return err
	}

	// if not bare and remote url is not empty and no remote exists
	// create default remote and pull from remote url
	remotes, err := c.r.Remotes()
	if err != nil {
		return err
	}
	if !c.opts.IsBare && c.opts.RemoteUrl != "" && len(remotes) == 0 {
		// attempt to get default remote
		if _, err := c.r.Remote(GitRemoteNameOrigin); err != nil {
			if err != git.ErrRemoteNotFound {
				return err
			}
			err = nil

			// create default remote
			if err := c.CreateRemote(GitRemoteNameOrigin, c.opts.RemoteUrl); err != nil {
				return err
			}

			// pull
			if err := c.Pull(GitRemoteNameOrigin); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *GitClient) InitMem() (err error) {
	// validate options
	if !c.opts.IsMem || c.opts.Path == "" {
		return ErrInvalidOptions
	}

	// get storage and worktree
	storage, wt, err := c.GetMemStorageAndMemFs(c.opts.Path)
	if err != nil {
		return err
	}

	// attempt to init
	c.r, err = git.Init(storage, wt)
	if err != nil {
		if err == git.ErrRepositoryAlreadyExists {
			// if already exists, attempt to open
			c.r, err = git.Open(storage, wt)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (c *GitClient) InitFs() (err error) {
	// validate options
	if c.opts.Path == "" {
		return ErrInvalidOptions
	}

	// get path
	path := c.opts.Path
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	// create directory if not exists
	_, err = os.Stat(path)
	if err != nil {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return err
		}
	}

	// check if .git exists
	if _, err := os.Stat(filepath.Join(path, git.GitDirName)); err != nil {
		// .git not exists, init
		c.r, err = git.PlainInit(path, c.opts.IsBare)
		if err != nil {
			return err
		}
	} else {
		// .git exists, open
		c.r, err = git.PlainOpen(path)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *GitClient) GetInitType() (res GitInitType) {
	if c.opts.IsMem {
		return GitInitTypeMem
	} else {
		return GitInitTypeFs
	}
}

func (c *GitClient) Checkout(args ...interface{}) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}

	// get arguments
	branch, hash, err := getBranchAndHashAndIsCreateFromArgs(args...)

	// get worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// create checkout options
	opts := git.CheckoutOptions{
		Branch: branch,
	}

	if hash.IsZero() {
		opts.Hash = hash
	} else {
		headRef, err := c.r.Head()
		if err != nil {
			return err
		}
		opts.Hash = headRef.Hash()
	}

	// checkout to the branch
	if err := wt.Checkout(&opts); err != nil {
		return err
	}

	return nil
}

func (c *GitClient) CheckoutBranch(branch string, args ...interface{}) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}

	// check if the branch exists
	if _, err := c.r.Branch(branch); err != nil {
		if err == git.ErrBranchNotFound {
			// create a new branch if it does not exist
			cfg := config.Branch{
				Name: branch,
			}
			if err := c.r.CreateBranch(&cfg); err != nil {
				return err
			}
			headRef, err := c.r.Head()
			if err == nil {
				ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), headRef.Hash())
				if err := c.r.Storer.SetReference(ref); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	return c.Checkout(branch, nil)
}

func (c *GitClient) CheckoutHash(hash string, args ...interface{}) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}

	return c.Checkout(nil, hash)
}

func (c *GitClient) Commit(msg string, args ...interface{}) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}

	// get worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// commit options
	opts := git.CommitOptions{
		All:       false,
		Author:    nil,
		Committer: nil,
		Parents:   nil,
		SignKey:   nil,
	}

	// commit
	if _, err := wt.Commit(msg, &opts); err != nil {
		return err
	}

	return nil
}

func (c *GitClient) CommitAll(msg string, args ...interface{}) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}

	// get worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// add all files
	if _, err := wt.Add("."); err != nil {
		return err
	}

	return c.Commit(msg, args...)
}

func (c *GitClient) Pull(args ...interface{}) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}

	// get remote name
	remoteName, err := getRemoteNameFromArgs(args...)
	if err != nil {
		return err
	}

	// get worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// pull options
	opts := git.PullOptions{
		RemoteName:   remoteName,
		SingleBranch: true,
	}

	// auth options
	auth, err := getGitAuth(c.opts.AuthType, c.opts.Username, c.opts.Password, c.opts.PrivateKeyPath)
	if err != nil {
		return err
	}
	if auth != nil {
		opts.Auth = auth
	}

	// pull
	if err := wt.Pull(&opts); err != nil {
		if err == transport.ErrEmptyRemoteRepository {
			return nil
		}
		if err == git.NoErrAlreadyUpToDate {
			return nil
		}
		return err
	}

	return nil
}

func (c *GitClient) Push(args ...interface{}) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}

	// get remote name
	remoteName, err := getRemoteNameFromArgs(args...)
	if err != nil {
		return err
	}

	// get remote
	remote, err := c.r.Remote(remoteName)
	if err != nil {
		return err
	}

	// push options
	opts := git.PushOptions{
		RemoteName: remoteName,
	}

	// auth options
	auth, err := getGitAuth(c.opts.AuthType, c.opts.Username, c.opts.Password, c.opts.PrivateKeyPath)
	if err != nil {
		return err
	}
	if auth != nil {
		opts.Auth = auth
	}

	// push to remote
	if err := remote.Push(&opts); err != nil {
		return err
	}

	return nil
}

func (c *GitClient) Reset(args ...interface{}) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}

	// get mode
	mode, err := getResetModeFromArgs(args...)
	if err != nil {
		return err
	}

	// get worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// reset
	if err := wt.Reset(&git.ResetOptions{
		Mode: mode,
	}); err != nil {
		return err
	}

	return nil
}

func (c *GitClient) Dispose(args ...interface{}) (err error) {
	switch c.GetInitType() {
	case GitInitTypeFs:
		path, err := filepath.Abs(c.opts.Path)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	case GitInitTypeMem:
		GitMemStorages.Delete(c.opts.Path)
		GitMemFileSystem.Delete(c.opts.Path)
	}
	return nil
}

func (c *GitClient) CreateRemote(remoteName string, url string) (err error) {
	if c.opts.IsBare {
		return ErrInvalidActionsForBareRepo
	}
	_, err = c.r.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{url},
	})
	if err != nil {
		return err
	}
	return
}

func (c *GitClient) GetLogs() (logs []GitLog, err error) {
	iter, err := c.r.Log(&git.LogOptions{
		All: true,
	})
	if err != nil {
		return logs, err
	}
	if err := iter.ForEach(func(commit *object.Commit) error {
		log := GitLog{
			Msg: commit.Message,
			//Branch:    commit.Committer,
			AuthorName:  commit.Author.Name,
			AuthorEmail: commit.Author.Email,
			Timestamp:   commit.Author.When,
		}
		logs = append(logs, log)
		return nil
	}); err != nil {
		return logs, err
	}
	return
}

func (c *GitClient) GetMemStorageAndMemFs(key string) (storage *memory.Storage, fs billy.Filesystem, err error) {
	// storage
	storageItem, ok := GitMemStorages.Load(key)
	if !ok {
		storage = memory.NewStorage()
		GitMemStorages.Store(key, storage)
	} else {
		switch storageItem.(type) {
		case *memory.Storage:
			storage = storageItem.(*memory.Storage)
		default:
			storage = memory.NewStorage()
			GitMemStorages.Store(key, storage)
		}
	}

	// file system
	fsItem, ok := GitMemFileSystem.Load(key)
	if !ok {
		fs = memfs.New()
		GitMemFileSystem.Store(key, fs)
	} else {
		switch fsItem.(type) {
		case billy.Filesystem:
			fs = fsItem.(billy.Filesystem)
		default:
			fs = memfs.New()
			GitMemFileSystem.Store(key, fs)
		}
	}

	return storage, fs, nil
}
