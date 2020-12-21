package vcs

import (
	"encoding/json"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type TestCredentials struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	TestRepoHttpUrl string `json:"test_repo_http_url"`
	SshUsername     string `json:"ssh_username"`
	SshPassword     string `json:"ssh_password"`
	TestRepoSshUrl  string `json:"test_repo_ssh_url"`
}

func setup() (err error) {
	if _, err := os.Stat("./tmp"); err == nil {
		if err := os.RemoveAll("./tmp"); err != nil {
			return err
		}
	}
	if err := os.MkdirAll("./tmp", os.ModePerm); err != nil {
		return err
	}
	return
}

func cleanup() (err error) {
	_, err = os.Stat("./tmp")
	if err == nil {
		err = os.RemoveAll("./tmp")
	}
	return
}

func TestNewGitClient(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// test with options
	c, err := NewGitClient(&GitOptions{
		Path:      "./tmp/test_repo",
		RemoteUrl: "test_url",
		IsBare:    true,
	})
	require.Nil(t, err)
	require.NotEmpty(t, c.r)
	require.NotEmpty(t, c.opts)
	require.Equal(t, "test_url", c.opts.RemoteUrl)
	require.True(t, c.opts.IsBare)

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}

func TestGitClient_Init(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// test not bare
	c, err := NewGitClient(&GitOptions{
		Path:      "./tmp/test_repo",
		RemoteUrl: "test_url",
		IsBare:    true,
	})
	require.Nil(t, err)
	require.NotEmpty(t, c.r)
	require.DirExists(t, "./tmp/test_repo")
	require.DirExists(t, "./tmp/test_repo/.git")

	// test bare
	c, err = NewGitClient(&GitOptions{
		Path:   "./tmp/test_repo_bare",
		IsBare: true,
	})
	require.Nil(t, err)
	require.NotEmpty(t, c.r)
	require.DirExists(t, "./tmp/test_repo_bare")
	files, err := ioutil.ReadDir("./tmp/test_repo_bare")
	require.Nil(t, err)
	require.Greater(t, len(files), 0)

	// test existing
	c, err = NewGitClient(&GitOptions{
		Path: "./tmp/test_repo",
	})
	require.Nil(t, err)
	require.NotEmpty(t, c.r)

	// test remote exists
	remotePath, err := filepath.Abs("./tmp/test_repo_bare")
	require.Nil(t, err)
	c, err = NewGitClient(&GitOptions{
		Path:      "./tmp/test_repo_with_remote",
		RemoteUrl: remotePath,
		IsBare:    false,
	})
	require.Nil(t, err)
	remote, err := c.r.Remote(GitRemoteNameOrigin)
	require.Nil(t, err)
	require.NotNil(t, remote)
	require.Equal(t, GitRemoteNameOrigin, remote.Config().Name)

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}

func TestGitClient_CheckoutBranch(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// create new git client
	c, err := NewGitClient(&GitOptions{
		Path: "./tmp/test_repo",
	})
	require.Nil(t, err)

	// test commit files
	content := "it works"
	err = ioutil.WriteFile("./tmp/test_repo/test_file.txt", []byte(content), os.ModePerm)
	require.Nil(t, err)
	err = c.CommitAll("initial commit")
	require.Nil(t, err)

	// test checkout branch
	err = c.CheckoutBranch("test")
	require.Nil(t, err)
	b, err := c.r.Branch("test")
	require.Nil(t, err)
	require.Equal(t, "test", b.Name)

	// check branches
	iter, err := c.r.Branches()
	require.Nil(t, err)
	var branches []string
	err = iter.ForEach(func(reference *plumbing.Reference) error {
		branches = append(branches, string(reference.Name()))
		return nil
	})
	require.Nil(t, err)
	require.Contains(t, branches, plumbing.NewBranchReferenceName("test").String())
	require.Greater(t, len(branches), 1)

	// checkout to master
	err = c.CheckoutBranch("master")
	require.Nil(t, err)
	b, err = c.r.Branch("master")
	require.Nil(t, err)
	require.Equal(t, "master", b.Name)

	// check branches
	iter, err = c.r.Branches()
	require.Nil(t, err)
	branches = []string{}
	err = iter.ForEach(func(reference *plumbing.Reference) error {
		branches = append(branches, string(reference.Name()))
		return nil
	})
	require.Nil(t, err)
	require.Contains(t, branches, plumbing.NewBranchReferenceName("master").String())
	require.Greater(t, len(branches), 1)

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}

func TestGitClient_CommitAll(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// create new git client
	c, err := NewGitClient(&GitOptions{
		Path: "./tmp/test_repo",
	})
	require.Nil(t, err)

	// test commit files
	content := "it works"
	err = ioutil.WriteFile("./tmp/test_repo/test_file.txt", []byte(content), os.ModePerm)
	require.Nil(t, err)
	err = c.CommitAll("initial commit")
	require.Nil(t, err)

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}

func TestGitClient_PushAndPullAndClone(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// create a remote repo
	c, err := NewGitClient(&GitOptions{
		Path:   "./tmp/test_repo_remote",
		IsBare: true,
	})
	require.Nil(t, err)

	// create a local repo
	remotePath, err := filepath.Abs("./tmp/test_repo_remote")
	require.Nil(t, err)
	c, err = NewGitClient(&GitOptions{
		Path:   "./tmp/test_repo_local",
		IsBare: false,
	})
	require.Nil(t, err)

	// test commit files
	content := "it works"
	err = ioutil.WriteFile("./tmp/test_repo_local/test_file.txt", []byte(content), os.ModePerm)
	require.Nil(t, err)
	err = c.CommitAll("initial commit")
	require.Nil(t, err)

	// create a second git client
	c2, err := NewGitClient(&GitOptions{
		Path:      "./tmp/test_repo_pull",
		RemoteUrl: remotePath,
		IsBare:    false,
	})
	require.Nil(t, err)

	// push to remote
	err = c.Push(nil)
	require.Nil(t, err)

	// pull to the second git client
	err = c2.Pull(nil)
	require.Nil(t, err)
	data, err := ioutil.ReadFile("./tmp/test_repo_pull/test_file.txt")
	require.Nil(t, err)
	require.Equal(t, content, string(data))

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}

func TestGitClient_Reset(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// create new git client
	c, err := NewGitClient(&GitOptions{
		Path: "./tmp/test_repo",
	})
	require.Nil(t, err)

	// test reset
	content := "it works"
	err = ioutil.WriteFile("./tmp/test_repo/test_file.txt", []byte(content), os.ModePerm)
	require.Nil(t, err)
	err = c.CommitAll("initial commit")
	require.Nil(t, err)
	err = ioutil.WriteFile("./tmp/test_repo/test_file_tmp.txt", []byte(content), os.ModePerm)
	require.Nil(t, err)
	err = c.Reset(git.HardReset) // git reset --hard
	require.Nil(t, err)
	_, err = os.Stat("./tmp/test_repo/test_file_tmp.txt")
	require.IsType(t, &os.PathError{}, err)

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}

func TestGitClient_GetLogs(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// create new git client
	c, err := NewGitClient(&GitOptions{
		Path: "./tmp/test_repo",
	})
	require.Nil(t, err)

	// test commit files
	content := "it works"
	err = ioutil.WriteFile("./tmp/test_repo/test_file.txt", []byte(content), os.ModePerm)
	require.Nil(t, err)
	err = c.CommitAll("initial commit")
	require.Nil(t, err)
	logs, err := c.GetLogs()
	require.Nil(t, err)
	require.Greater(t, len(logs), 0)

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}

func TestGitClient_InitWithHttpAuth(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// get credentials
	var cred TestCredentials
	data, err := ioutil.ReadFile("credentials.json")
	require.Nil(t, err)
	err = json.Unmarshal(data, &cred)
	require.Nil(t, err)

	// create new git client
	c, err := NewGitClient(&GitOptions{
		Path:      "./tmp/test_repo",
		RemoteUrl: cred.TestRepoHttpUrl,
		IsBare:    false,
		AuthType:  GitAuthTypeHTTP,
		Username:  cred.Username,
		Password:  cred.Password,
	})
	require.Nil(t, err)
	require.NotNil(t, c.r)
	files, err := ioutil.ReadDir("./tmp/test_repo")
	require.Greater(t, len(files), 0)
	data, err = ioutil.ReadFile("./tmp/test_repo/README.md")
	require.Nil(t, err)
	require.Contains(t, string(data), "Test Repo")
	fmt.Println(string(data))

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}

func TestGitClient_InitWithSshAuth(t *testing.T) {
	// setup
	err := setup()
	require.Nil(t, err)

	// get credentials
	var cred TestCredentials
	data, err := ioutil.ReadFile("credentials.json")
	require.Nil(t, err)
	err = json.Unmarshal(data, &cred)
	require.Nil(t, err)

	// create new git client
	c, err := NewGitClient(&GitOptions{
		Path:      "./tmp/test_repo",
		RemoteUrl: cred.TestRepoSshUrl,
		IsBare:    false,
		AuthType:  GitAuthTypeSSH,
		Username:  cred.SshUsername,
		Password:  cred.SshPassword,
	})
	require.Nil(t, err)
	require.NotNil(t, c.r)
	files, err := ioutil.ReadDir("./tmp/test_repo")
	require.Greater(t, len(files), 0)
	data, err = ioutil.ReadFile("./tmp/test_repo/README.md")
	require.Nil(t, err)
	require.Contains(t, string(data), "Test Repo")
	fmt.Println(string(data))

	// cleanup
	err = cleanup()
	require.Nil(t, err)
}
