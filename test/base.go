package test

import (
	vcs "github.com/crawlab-team/crawlab-vcs"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"
	"time"
)

func init() {
	var err error
	T, err = NewTest()
	if err != nil {
		panic(err)
	}
}

var T *Test

type Test struct {
	RemoteRepoPath           string
	LocalRepoPath            string
	LocalRepo                *vcs.GitClient
	FsRepoPath               string
	MemRepoPath              string
	AuthRepoPath             string
	TestFileName             string
	TestFileContent          string
	TestBranchName           string
	TestCommitMessage        string
	InitialCommitMessage     string
	InitialReadmeFileName    string
	InitialReadmeFileContent string
}

func (t *Test) Setup(t2 *testing.T) {
	var err error

	// remote repo
	if err := vcs.CreateBareGitRepo(t.RemoteRepoPath); err != nil {
		panic(err)
	}

	// local repo (fs)
	t.LocalRepo, err = vcs.NewGitClient(
		vcs.WithPath(t.LocalRepoPath),
		vcs.WithRemoteUrl(t.RemoteRepoPath),
	)
	if err != nil {
		panic(err)
	}

	// initial commit
	filePath := path.Join(t.LocalRepoPath, t.InitialReadmeFileContent)
	if err := ioutil.WriteFile(filePath, []byte(t.InitialReadmeFileContent), os.FileMode(0766)); err != nil {
		panic(err)
	}
	if err := t.LocalRepo.CommitAll(t.InitialCommitMessage); err != nil {
		panic(err)
	}

	t2.Cleanup(t.Cleanup)
}

func (t *Test) Cleanup() {
	if err := T.LocalRepo.Dispose(); err != nil {
		panic(err)
	}
	if err := os.RemoveAll(T.RemoteRepoPath); err != nil {
		panic(err)
	}

	vcs.GitMemStorages = sync.Map{}
	vcs.GitMemFileSystem = sync.Map{}

	// wait to avoid caching
	time.Sleep(500 * time.Millisecond)
}

func NewTest() (t *Test, err error) {
	t = &Test{}

	// clear tmp directory
	_ = os.RemoveAll("./tmp")
	_ = os.MkdirAll("./tmp", os.FileMode(0766))

	// remote repo path
	t.RemoteRepoPath = "./tmp/test_remote_repo"

	// local repo path
	t.LocalRepoPath = "./tmp/test_local_repo"

	// fs repo path
	t.FsRepoPath = "./tmp/test_fs_repo"

	// mem repo path
	t.MemRepoPath = "./tmp/test_mem_repo"

	// auth repo path
	t.AuthRepoPath = "./tmp/test_auth_repo"

	// test file name
	t.TestFileName = "test_file.txt"

	// test file content
	t.TestFileContent = "it works"

	// test branch name
	t.TestBranchName = "develop"

	// test commit message
	t.InitialCommitMessage = "test commit"

	// initial commit message
	t.InitialCommitMessage = "initial commit"

	// initial readme file name
	t.InitialReadmeFileName = "README.md"

	// initial readme file content
	t.InitialReadmeFileContent = "README"

	return t, nil
}
