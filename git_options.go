package vcs

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"golang.org/x/crypto/openpgp"
)

type GitOption func(c *GitClient)

type GitCheckoutOption func(o *git.CheckoutOptions)

func WithBranch(branch string) GitCheckoutOption {
	return func(o *git.CheckoutOptions) {
		o.Branch = plumbing.NewBranchReferenceName(branch)
	}
}

func WithHash(hash string) GitCheckoutOption {
	return func(o *git.CheckoutOptions) {
		h := plumbing.NewHash(hash)
		if h.IsZero() {
			return
		}
		o.Hash = h
	}
}

type GitCommitOption func(o *git.CommitOptions)

func WithAll(all bool) GitCommitOption {
	return func(o *git.CommitOptions) {
		o.All = all
	}
}

func WithAuthor(author *object.Signature) GitCommitOption {
	return func(o *git.CommitOptions) {
		o.Author = author
	}
}

func WithCommitter(committer *object.Signature) GitCommitOption {
	return func(o *git.CommitOptions) {
		o.Committer = committer
	}
}

func WithParents(parents []plumbing.Hash) GitCommitOption {
	return func(o *git.CommitOptions) {
		o.Parents = parents
	}
}

func WithSignKey(signKey *openpgp.Entity) GitCommitOption {
	return func(o *git.CommitOptions) {
		o.SignKey = signKey
	}
}

type GitPullOption func(o *git.PullOptions)

func WithRemoteNamePull(name string) GitPullOption {
	return func(o *git.PullOptions) {
		o.RemoteName = name
	}
}

func WithReferenceNamePull(branch string) GitPullOption {
	return func(o *git.PullOptions) {
		o.ReferenceName = plumbing.NewBranchReferenceName(branch)
	}
}

func WithDepth(depth int) GitPullOption {
	return func(o *git.PullOptions) {
		o.Depth = depth
	}
}

func WithAuthPull(auth transport.AuthMethod) GitPullOption {
	return func(o *git.PullOptions) {
		if auth != nil {
			o.Auth = auth
		}
	}
}

func WithRecurseSubmodules(recurseSubmodules git.SubmoduleRescursivity) GitPullOption {
	return func(o *git.PullOptions) {
		o.RecurseSubmodules = recurseSubmodules
	}
}

func WithForcePull(force bool) GitPullOption {
	return func(o *git.PullOptions) {
		o.Force = force
	}
}

type GitPushOption func(o *git.PushOptions)

func WithRemoteNamePush(name string) GitPushOption {
	return func(o *git.PushOptions) {
		o.RemoteName = name
	}
}

func WithRefSpecs(specs []config.RefSpec) GitPushOption {
	return func(o *git.PushOptions) {
		o.RefSpecs = specs
	}
}

func WithAuthPush(auth transport.AuthMethod) GitPushOption {
	return func(o *git.PushOptions) {
		o.Auth = auth
	}
}

func WithPrune(prune bool) GitPushOption {
	return func(o *git.PushOptions) {
		o.Prune = prune
	}
}

func WithForcePush(force bool) GitPushOption {
	return func(o *git.PushOptions) {
		o.Force = force
	}
}

type GitResetOption func(o *git.ResetOptions)

func WithCommit(commit plumbing.Hash) GitResetOption {
	return func(o *git.ResetOptions) {
		o.Commit = commit
	}
}

func WithMode(mode git.ResetMode) GitResetOption {
	return func(o *git.ResetOptions) {
		o.Mode = mode
	}
}
