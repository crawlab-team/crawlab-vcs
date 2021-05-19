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
	"path/filepath"
)

type GitClient struct {
	// settings
	path           string
	remoteUrl      string
	isMem          bool
	authType       GitAuthType
	username       string
	password       string
	privateKeyPath string

	// internals
	r *git.Repository
}

func (c *GitClient) Init() (err error) {
	initType := c.getInitType()
	switch initType {
	case GitInitTypeFs:
		err = c.initFs()
	case GitInitTypeMem:
		err = c.initMem()
	}
	if err != nil {
		return err
	}

	// if remote url is not empty and no remote exists
	// create default remote and pull from remote url
	remotes, err := c.r.Remotes()
	if err != nil {
		return err
	}
	if c.remoteUrl != "" && len(remotes) == 0 {
		// attempt to get default remote
		if _, err := c.r.Remote(GitRemoteNameOrigin); err != nil {
			if err != git.ErrRemoteNotFound {
				return err
			}
			err = nil

			// create default remote
			if err := c.createRemote(GitRemoteNameOrigin, c.remoteUrl); err != nil {
				return err
			}

			// pull
			opts := []GitPullOption{
				WithRemoteNamePull(GitRemoteNameOrigin),
			}
			if err := c.Pull(opts...); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *GitClient) Dispose() (err error) {
	switch c.getInitType() {
	case GitInitTypeFs:
		path, err := filepath.Abs(c.path)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	case GitInitTypeMem:
		GitMemStorages.Delete(c.path)
		GitMemFileSystem.Delete(c.path)
	}
	return nil
}

func (c *GitClient) Checkout(opts ...GitCheckoutOption) (err error) {
	// worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// apply options
	o := &git.CheckoutOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// checkout to the branch
	if err := wt.Checkout(o); err != nil {
		return err
	}

	return nil
}

func (c *GitClient) Commit(msg string, opts ...GitCommitOption) (err error) {
	// worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// apply options
	o := &git.CommitOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// commit
	if _, err := wt.Commit(msg, o); err != nil {
		return err
	}

	return nil
}

func (c *GitClient) Pull(opts ...GitPullOption) (err error) {
	// worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// auth
	auth, err := c.getGitAuth(c.authType, c.username, c.password, c.privateKeyPath)
	if err != nil {
		return err
	}
	if auth != nil {
		opts = append(opts, WithAuthPull(auth))
	}

	// apply options
	o := &git.PullOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// pull
	if err := wt.Pull(o); err != nil {
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

func (c *GitClient) Push(opts ...GitPushOption) (err error) {
	// auth
	auth, err := c.getGitAuth(c.authType, c.username, c.password, c.privateKeyPath)
	if err != nil {
		return err
	}
	if auth != nil {
		opts = append(opts, WithAuthPush(auth))
	}

	// apply options
	o := &git.PushOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// push
	if err := c.r.Push(o); err != nil {
		return err
	}

	return nil
}

func (c *GitClient) Reset(opts ...GitResetOption) (err error) {
	// apply options
	o := &git.ResetOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// reset
	if err := wt.Reset(o); err != nil {
		return err
	}

	return nil
}

func (c *GitClient) CheckoutBranch(branch string, opts ...GitCheckoutOption) (err error) {
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

	// add to options
	opts = append(opts, WithBranch(branch))

	return c.Checkout(opts...)
}

func (c *GitClient) CheckoutHash(hash string, opts ...GitCheckoutOption) (err error) {
	// add to options
	opts = append(opts, WithHash(hash))

	return c.Checkout(opts...)
}

func (c *GitClient) CommitAll(msg string, opts ...GitCommitOption) (err error) {
	// worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return err
	}

	// add all files
	if _, err := wt.Add("."); err != nil {
		return err
	}

	return c.Commit(msg, opts...)
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

func (c *GitClient) initMem() (err error) {
	// validate options
	if !c.isMem || c.path == "" {
		return ErrInvalidOptions
	}

	// get storage and worktree
	storage, wt, err := c.getMemStorageAndMemFs(c.path)
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

func (c *GitClient) initFs() (err error) {
	// validate options
	if c.path == "" {
		return ErrInvalidOptions
	}

	// get path
	path := c.path
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
		err = nil
	}

	// try to open repo
	c.r, err = git.PlainOpen(path)
	if err == git.ErrRepositoryNotExists {
		// repo not exists, init
		c.r, err = git.PlainInit(path, false)
		if err != nil {
			return err
		}
		err = nil
	} else if err != nil {
		// error
		return err
	}

	return nil
}

func (c *GitClient) getInitType() (res GitInitType) {
	if c.isMem {
		return GitInitTypeMem
	} else {
		return GitInitTypeFs
	}
}

func (c *GitClient) createRemote(remoteName string, url string) (err error) {
	_, err = c.r.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{url},
	})
	if err != nil {
		return err
	}
	return
}

func (c *GitClient) getMemStorageAndMemFs(key string) (storage *memory.Storage, fs billy.Filesystem, err error) {
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

func (c *GitClient) getBranchAndHashAndIsCreateFromArgs(args ...interface{}) (branch plumbing.ReferenceName, hash plumbing.Hash, err error) {
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

func (c *GitClient) getRemoteNameFromArgs(args ...interface{}) (remoteName string, err error) {
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

func (c *GitClient) getResetModeFromArgs(args ...interface{}) (mode git.ResetMode, err error) {
	if len(args) < 1 {
		return mode, ErrInvalidArgsLength
	}
	if args[0] != nil {
		mode, err = c.getResetMode(args[0])
		if err != nil {
			return mode, err
		}
	}
	return
}

func (c *GitClient) getResetMode(mode interface{}) (res git.ResetMode, err error) {
	switch mode.(type) {
	case int8:
		return git.ResetMode(int8(0)), nil
	case git.ResetMode:
		return mode.(git.ResetMode), err
	}
	return git.MixedReset, ErrUnsupportedType
}

func (c *GitClient) getGitAuth(authType GitAuthType, username, password, privateKeyPath string) (auth transport.AuthMethod, err error) {
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
			return nil, err
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
		return nil, ErrUnsupportedType
	}
	return auth, nil
}

func NewGitClient(opts ...GitOption) (c *GitClient, err error) {
	// client
	c = &GitClient{
		isMem:          false,
		authType:       GitAuthTypeNone,
		privateKeyPath: getDefaultPublicKeyPath(),
	}

	// apply options
	for _, opt := range opts {
		opt(c)
	}

	// init
	if err := c.Init(); err != nil {
		return c, err
	}

	return
}
