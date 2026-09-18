package gitops

import (
	"context"

	"github.com/google/go-github/v75/github"
)

type gitClient interface {
	GetRef(
		ctx context.Context, owner, repo, ref string,
	) (*github.Reference, *github.Response, error)
	CreateRef(
		ctx context.Context, owner, repo string, ref github.CreateRef,
	) (*github.Reference, *github.Response, error)
	GetCommit(
		ctx context.Context, owner, repo, sha string,
	) (*github.Commit, *github.Response, error)
	CreateTree(
		ctx context.Context, owner, repo, baseTree string, entries []*github.TreeEntry,
	) (*github.Tree, *github.Response, error)
	CreateCommit(
		ctx context.Context, owner, repo string, commit github.Commit, opts *github.CreateCommitOptions,
	) (*github.Commit, *github.Response, error)
	UpdateRef(
		ctx context.Context, owner, repo, ref string, updateRef github.UpdateRef,
	) (*github.Reference, *github.Response, error)
}

type prClient interface {
	List(
		ctx context.Context, owner, repo string, opts *github.PullRequestListOptions,
	) ([]*github.PullRequest, *github.Response, error)
	Create(
		ctx context.Context, owner, repo string, pull *github.NewPullRequest,
	) (*github.PullRequest, *github.Response, error)
}

type repoClient interface {
	Get(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
}

type githubGitClient struct {
	git *github.GitService
}

func (c githubGitClient) GetRef(
	ctx context.Context, owner, repo, ref string,
) (*github.Reference, *github.Response, error) {
	return c.git.GetRef(ctx, owner, repo, ref)
}

func (c githubGitClient) CreateRef(
	ctx context.Context,
	owner, repo string,
	ref github.CreateRef,
) (*github.Reference, *github.Response, error) {
	return c.git.CreateRef(ctx, owner, repo, ref)
}

func (c githubGitClient) GetCommit(
	ctx context.Context, owner, repo, sha string,
) (*github.Commit, *github.Response, error) {
	return c.git.GetCommit(ctx, owner, repo, sha)
}

func (c githubGitClient) CreateTree(
	ctx context.Context,
	owner, repo, baseTree string,
	entries []*github.TreeEntry,
) (*github.Tree, *github.Response, error) {
	return c.git.CreateTree(ctx, owner, repo, baseTree, entries)
}

func (c githubGitClient) CreateCommit(
	ctx context.Context,
	owner, repo string,
	commit github.Commit,
	opts *github.CreateCommitOptions,
) (*github.Commit, *github.Response, error) {
	return c.git.CreateCommit(ctx, owner, repo, commit, opts)
}

func (c githubGitClient) UpdateRef(
	ctx context.Context,
	owner, repo, ref string,
	updateRef github.UpdateRef,
) (*github.Reference, *github.Response, error) {
	return c.git.UpdateRef(ctx, owner, repo, ref, updateRef)
}

// installationClients bundles the GitHub API surfaces PublishManifests/TestConnection need,
// all built from a single authenticated *github.Client for one GitHub App installation.
type installationClients struct {
	git  gitClient
	pr   prClient
	repo repoClient
}
