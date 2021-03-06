package vcs

import (
	"github.com/crawlab-team/go-trace"
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
)

type GitClient struct {
	// settings
	path           string
	remoteUrl      string
	isMem          bool
	authType       GitAuthType
	username       string
	password       string
	privateKey     string
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
				return trace.TraceError(err)
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
		if err := os.RemoveAll(c.path); err != nil {
			return trace.TraceError(err)
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
		return trace.TraceError(err)
	}

	// apply options
	o := &git.CheckoutOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// checkout to the branch
	if err := wt.Checkout(o); err != nil {
		return trace.TraceError(err)
	}

	return nil
}

func (c *GitClient) Commit(msg string, opts ...GitCommitOption) (err error) {
	// worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return trace.TraceError(err)
	}

	// apply options
	o := &git.CommitOptions{}
	for _, opt := range opts {
		opt(o)
	}

	// commit
	if _, err := wt.Commit(msg, o); err != nil {
		return trace.TraceError(err)
	}

	return nil
}

func (c *GitClient) Pull(opts ...GitPullOption) (err error) {
	// worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return trace.TraceError(err)
	}

	// auth
	auth, err := c.getGitAuth()
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
		return trace.TraceError(err)
	}

	return nil
}

func (c *GitClient) Push(opts ...GitPushOption) (err error) {
	// auth
	auth, err := c.getGitAuth()
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
		return trace.TraceError(err)
	}

	return nil
}

func (c *GitClient) Reset(opts ...GitResetOption) (err error) {
	// apply options
	o := &git.ResetOptions{
		Mode: git.HardReset,
	}
	for _, opt := range opts {
		opt(o)
	}

	// worktree
	wt, err := c.r.Worktree()
	if err != nil {
		return trace.TraceError(err)
	}

	// reset
	if err := wt.Reset(o); err != nil {
		return trace.TraceError(err)
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
				return trace.TraceError(err)
			}

			// HEAD reference
			headRef, err := c.r.Head()
			if err != nil {
				return trace.TraceError(err)
			}

			// branch reference name
			branchRefName := plumbing.NewBranchReferenceName(branch)

			// branch reference
			ref := plumbing.NewHashReference(branchRefName, headRef.Hash())

			// set HEAD to branch reference
			if err := c.r.Storer.SetReference(ref); err != nil {
				return trace.TraceError(err)
			}
		} else {
			return trace.TraceError(err)
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
		return trace.TraceError(err)
	}

	// add all files
	if _, err := wt.Add("."); err != nil {
		return trace.TraceError(err)
	}

	return c.Commit(msg, opts...)
}

func (c *GitClient) GetLogs() (logs []GitLog, err error) {
	iter, err := c.r.Log(&git.LogOptions{
		All: true,
	})
	if err != nil {
		return nil, trace.TraceError(err)
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
		return nil, trace.TraceError(err)
	}
	return
}

func (c *GitClient) GetRepository() (r *git.Repository) {
	return c.r
}

func (c *GitClient) GetPath() (path string) {
	return c.path
}

func (c *GitClient) GetRemoteUrl() (path string) {
	return c.remoteUrl
}

func (c *GitClient) GetIsMem() (isMem bool) {
	return c.isMem
}

func (c *GitClient) GetAuthType() (authType GitAuthType) {
	return c.authType
}

func (c *GitClient) GetUsername() (username string) {
	return c.username
}

func (c *GitClient) GetPrivateKeyPath() (path string) {
	return c.privateKeyPath
}

func (c *GitClient) GetCurrentBranch() (branch string, err error) {
	headRef, err := c.r.Head()
	if err != nil {
		return "", trace.TraceError(err)
	}
	if !headRef.Name().IsBranch() {
		return "", trace.TraceError(ErrUnableToGetCurrentBranch)
	}
	return headRef.Name().String(), nil
}

func (c *GitClient) initMem() (err error) {
	// validate options
	if !c.isMem || c.path == "" {
		return trace.TraceError(ErrInvalidOptions)
	}

	// get storage and worktree
	storage, wt := c.getMemStorageAndMemFs(c.path)

	// attempt to init
	c.r, err = git.Init(storage, wt)
	if err != nil {
		if err == git.ErrRepositoryAlreadyExists {
			// if already exists, attempt to open
			c.r, err = git.Open(storage, wt)
			if err != nil {
				return trace.TraceError(err)
			}
		} else {
			return trace.TraceError(err)
		}
	}

	return nil
}

func (c *GitClient) initFs() (err error) {
	// validate options
	if c.path == "" {
		return trace.TraceError(ErrInvalidOptions)
	}

	// create directory if not exists
	_, err = os.Stat(c.path)
	if err != nil {
		if err := os.MkdirAll(c.path, os.ModePerm); err != nil {
			return trace.TraceError(err)
		}
		err = nil
	}

	// try to open repo
	c.r, err = git.PlainOpen(c.path)
	if err == git.ErrRepositoryNotExists {
		// repo not exists, init
		c.r, err = git.PlainInit(c.path, false)
		if err != nil {
			return trace.TraceError(err)
		}
	} else if err != nil {
		// error
		return trace.TraceError(err)
	}

	return nil
}

func (c *GitClient) clone() (err error) {
	// validate
	if c.remoteUrl == "" {
		return trace.TraceError(ErrUnableToCloneWithEmptyRemoteUrl)
	}

	// auth
	auth, err := c.getGitAuth()
	if err != nil {
		return err
	}

	// options
	o := &git.CloneOptions{
		URL:  c.remoteUrl,
		Auth: auth,
	}

	// clone
	if _, err := git.PlainClone(c.path, false, o); err != nil {
		return trace.TraceError(err)
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
		return trace.TraceError(err)
	}
	return
}

func (c *GitClient) getMemStorageAndMemFs(key string) (storage *memory.Storage, fs billy.Filesystem) {
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

	return storage, fs
}

func (c *GitClient) getGitAuth() (auth transport.AuthMethod, err error) {
	switch c.authType {
	case GitAuthTypeNone:
		return nil, nil
	case GitAuthTypeHTTP:
		auth = &http.BasicAuth{
			Username: c.username,
			Password: c.password,
		}
		return auth, nil
	case GitAuthTypeSSH:
		var privateKeyData []byte
		if c.privateKey != "" {
			// private key content
			privateKeyData = []byte(c.privateKey)
		} else if c.privateKeyPath != "" {
			// read from private key file
			privateKeyData, err = ioutil.ReadFile(c.privateKeyPath)
			if err != nil {
				return nil, trace.TraceError(err)
			}
		} else {
			// no private key
			return nil, nil
		}
		var signer ssh.Signer
		if c.password != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(privateKeyData, []byte(c.password))
		} else {
			signer, err = ssh.ParsePrivateKey(privateKeyData)
		}
		if err != nil {
			return nil, trace.TraceError(err)
		}
		auth = &gitssh.PublicKeys{
			User:   c.username,
			Signer: signer,
			HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		}
		return auth, nil
	default:
		return nil, trace.TraceError(ErrInvalidAuthType)
	}
}

func NewGitClient(opts ...GitOption) (c *GitClient, err error) {
	// client
	c = &GitClient{
		isMem:          false,
		authType:       GitAuthTypeNone,
		username:       "git",
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
