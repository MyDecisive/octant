package gitops

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v75/github"
	"github.com/mydecisive/octant/internal/integration"
)

const (
	gitHubFileMode = "100644"

	// defaultIntegrationName is the name Octant stores the (single, cluster-wide) GitHub App
	// integration under. The CI/CD repository a GitOps GitHub App publishes to is a shared,
	// cluster-level setting rather than something configured per-connection.
	defaultIntegrationName = "default"

	defaultBaseBranch    = "main"
	defaultBranchPrefix  = "octant/manifests"
	defaultBasePath      = "octant"
	defaultCommitterName = "octant"

	commitMessageFmt    = "chore: update %s manifests"
	pullRequestTitleFmt = "chore: update %s manifests"
	pullRequestBodyFmt  = "Automated manifest update for connection `%s` in namespace `%s`, opened by Octant so ArgoCD can reconcile it after review/merge." // nolint:lll
)

var (
	ErrGitHubAppConfig = errors.New("github app config")
	ErrInvalidPath     = errors.New("invalid git path")
)

// APIClient tests ad-hoc GitHub App credentials without persisting them.
type APIClient interface {
	TestConnection(ctx context.Context, creds integration.GitHubAppIntegrationData) (bool, error)
}

// Publisher opens (or updates) a branch containing generated manifests and ensures a pull
// request exists against the configured base branch, for ArgoCD to reconcile once merged.
type Publisher interface {
	// PublishManifests returns a nil result and nil error when no GitHub App integration is
	// configured, so callers can treat GitOps publishing as an optional, best-effort step.
	PublishManifests(ctx context.Context, input PublishInput) (*PublishResult, error)
}

type PublishInput struct {
	ConnectionName string
	Namespace      string
	MDAIVersion    string
	Manifests      map[string][]byte
}

type PublishResult struct {
	Branch            string
	CommitSHA         string
	CommitURL         string
	PullRequestURL    string
	PullRequestNumber int
	Files             int
}

// GitHubManifestPublisher authenticates as a GitHub App installation to open GitOps pull
// requests against a CI/CD repository that ArgoCD watches. Credentials are looked up per-call
// from the integration store so they can be managed from octant-ui at runtime rather than baked
// into static process configuration.
type GitHubManifestPublisher struct {
	integration integration.Integration[integration.GitHubAppIntegrationData]
	newClients  func(creds integration.GitHubAppIntegrationData) (installationClients, error)
}

var (
	_ Publisher = (*GitHubManifestPublisher)(nil)
	_ APIClient = (*GitHubManifestPublisher)(nil)
)

func NewGitHubManifestPublisher(
	githubIntegration integration.Integration[integration.GitHubAppIntegrationData],
) *GitHubManifestPublisher {
	return &GitHubManifestPublisher{
		integration: githubIntegration,
		newClients:  newInstallationClients,
	}
}

func newInstallationClients(creds integration.GitHubAppIntegrationData) (installationClients, error) {
	if creds.PrivateKey == "" {
		return installationClients{}, fmt.Errorf("%w: privateKey is required", ErrGitHubAppConfig)
	}
	privateKey := strings.ReplaceAll(creds.PrivateKey, `\n`, "\n")
	transport, err := ghinstallation.New(http.DefaultTransport, creds.AppID, creds.InstallationID, []byte(privateKey))
	if err != nil {
		return installationClients{}, fmt.Errorf("%w: create transport: %w", ErrGitHubAppConfig, err)
	}
	client := github.NewClient(&http.Client{Transport: transport})
	return installationClients{
		git:  githubGitClient{git: client.Git},
		pr:   client.PullRequests,
		repo: client.Repositories,
	}, nil
}

// TestConnection verifies that the given (not-yet-saved) credentials can authenticate and read
// the target repository, without persisting anything or touching the repository's contents.
func (p *GitHubManifestPublisher) TestConnection(
	ctx context.Context,
	creds integration.GitHubAppIntegrationData,
) (bool, error) {
	clients, err := p.newClients(creds)
	if err != nil {
		return false, err
	}
	if _, _, err := clients.repo.Get(ctx, creds.Owner, creds.Repository); err != nil {
		return false, fmt.Errorf("get repository %s/%s: %w", creds.Owner, creds.Repository, err)
	}
	return true, nil
}

func (p *GitHubManifestPublisher) PublishManifests(
	ctx context.Context,
	input PublishInput,
) (*PublishResult, error) {
	creds, err := p.integration.GetIntegrationByName(ctx, defaultIntegrationName)
	if err != nil {
		return nil, fmt.Errorf("get github app integration: %w", err)
	}
	if creds == nil {
		return nil, nil // nolint:nilnil // GitOps GitHub App integration not configured; nothing to do.
	}
	base := baseBranchOf(*creds)
	if len(input.Manifests) == 0 {
		return &PublishResult{Branch: base}, nil
	}

	clients, err := p.newClients(*creds)
	if err != nil {
		return nil, err
	}

	entries, err := treeEntries(*creds, input)
	if err != nil {
		return nil, err
	}

	baseRef, _, err := clients.git.GetRef(ctx, creds.Owner, creds.Repository, "heads/"+base)
	if err != nil {
		return nil, fmt.Errorf("get heads/%s ref: %w", base, err)
	}
	baseCommitSHA := baseRef.GetObject().GetSHA()

	baseCommit, _, err := clients.git.GetCommit(ctx, creds.Owner, creds.Repository, baseCommitSHA)
	if err != nil {
		return nil, fmt.Errorf("get base commit %s: %w", baseCommitSHA, err)
	}

	tree, _, err := clients.git.CreateTree(ctx, creds.Owner, creds.Repository, baseCommit.GetTree().GetSHA(), entries)
	if err != nil {
		return nil, fmt.Errorf("create manifest tree: %w", err)
	}

	author := commitAuthorOf(*creds)
	commit, _, err := clients.git.CreateCommit(ctx, creds.Owner, creds.Repository, github.Commit{
		Message:   github.Ptr(fmt.Sprintf(commitMessageFmt, input.ConnectionName)),
		Tree:      tree,
		Parents:   []*github.Commit{{SHA: github.Ptr(baseCommitSHA)}},
		Author:    author,
		Committer: author,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("create manifest commit: %w", err)
	}

	featureBranch := branchNameOf(*creds, input)
	if branchErr := upsertBranch(
		ctx, clients.git, creds.Owner, creds.Repository, featureBranch, commit.GetSHA(),
	); branchErr != nil {
		return nil, branchErr
	}

	pr, err := findOrCreatePullRequest(ctx, clients.pr, *creds, input, featureBranch, base)
	if err != nil {
		return nil, err
	}

	return &PublishResult{
		Branch:            featureBranch,
		CommitSHA:         commit.GetSHA(),
		CommitURL:         commit.GetHTMLURL(),
		PullRequestURL:    pr.GetHTMLURL(),
		PullRequestNumber: pr.GetNumber(),
		Files:             len(entries),
	}, nil
}

// upsertBranch points featureBranch at commitSHA, creating the ref if it doesn't exist yet or
// fast-forwarding/rewriting it (force) if Octant already opened it for a previous publish.
func upsertBranch(ctx context.Context, git gitClient, owner, repo, featureBranch, commitSHA string) error {
	refName := "heads/" + featureBranch
	_, resp, err := git.GetRef(ctx, owner, repo, refName)
	switch {
	case err == nil:
		if _, _, updateErr := git.UpdateRef(ctx, owner, repo, refName, github.UpdateRef{
			SHA:   commitSHA,
			Force: github.Ptr(true),
		}); updateErr != nil {
			return fmt.Errorf("update heads/%s ref: %w", featureBranch, updateErr)
		}
		return nil
	case resp != nil && resp.StatusCode == http.StatusNotFound:
		if _, _, createErr := git.CreateRef(ctx, owner, repo, github.CreateRef{
			Ref: "refs/" + refName,
			SHA: commitSHA,
		}); createErr != nil {
			return fmt.Errorf("create heads/%s ref: %w", featureBranch, createErr)
		}
		return nil
	default:
		return fmt.Errorf("get heads/%s ref: %w", featureBranch, err)
	}
}

// findOrCreatePullRequest reuses an already-open PR for featureBranch if Octant opened one on a
// previous publish, otherwise it opens a new one.
func findOrCreatePullRequest(
	ctx context.Context,
	prs prClient,
	creds integration.GitHubAppIntegrationData,
	input PublishInput,
	featureBranch, base string,
) (*github.PullRequest, error) {
	existing, _, err := prs.List(ctx, creds.Owner, creds.Repository, &github.PullRequestListOptions{
		State: "open",
		Head:  creds.Owner + ":" + featureBranch,
		Base:  base,
	})
	if err != nil {
		return nil, fmt.Errorf("list pull requests for heads/%s: %w", featureBranch, err)
	}
	if len(existing) > 0 {
		return existing[0], nil
	}

	pr, _, err := prs.Create(ctx, creds.Owner, creds.Repository, &github.NewPullRequest{
		Title: github.Ptr(fmt.Sprintf(pullRequestTitleFmt, input.ConnectionName)),
		Head:  github.Ptr(featureBranch),
		Base:  github.Ptr(base),
		Body:  github.Ptr(fmt.Sprintf(pullRequestBodyFmt, input.ConnectionName, input.Namespace)),
	})
	if err != nil {
		return nil, fmt.Errorf("create pull request for heads/%s: %w", featureBranch, err)
	}
	return pr, nil
}

func treeEntries(creds integration.GitHubAppIntegrationData, input PublishInput) ([]*github.TreeEntry, error) {
	entries := make([]*github.TreeEntry, 0, len(input.Manifests))
	for filename, content := range input.Manifests {
		manifestPath, err := manifestPathOf(creds, input, filename)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &github.TreeEntry{
			Path:    github.Ptr(manifestPath),
			Mode:    github.Ptr(gitHubFileMode),
			Type:    github.Ptr("blob"),
			Content: github.Ptr(string(content)),
		})
	}
	return entries, nil
}

func commitAuthorOf(creds integration.GitHubAppIntegrationData) *github.CommitAuthor {
	name := creds.CommitterName
	if name == "" {
		name = defaultCommitterName
	}
	if name == "" && creds.CommitterEmail == "" {
		return nil
	}
	return &github.CommitAuthor{
		Name:  github.Ptr(name),
		Email: github.Ptr(creds.CommitterEmail),
	}
}

func baseBranchOf(creds integration.GitHubAppIntegrationData) string {
	if creds.BaseBranch != "" {
		return creds.BaseBranch
	}
	return defaultBaseBranch
}

// branchNameOf deterministically names the feature branch Octant opens per-connection, so
// repeated publishes for the same connection update the same branch/PR instead of piling up
// new ones.
func branchNameOf(creds integration.GitHubAppIntegrationData, input PublishInput) string {
	prefix := creds.BranchPrefix
	if prefix == "" {
		prefix = defaultBranchPrefix
	}
	return fmt.Sprintf("%s/%s-%s", strings.Trim(prefix, "/"), input.Namespace, input.ConnectionName)
}

func manifestPathOf(creds integration.GitHubAppIntegrationData, input PublishInput, filename string) (string, error) {
	basePath := creds.BasePath
	if basePath == "" {
		basePath = defaultBasePath
	}
	base, err := cleanPath(basePath)
	if err != nil {
		return "", fmt.Errorf("%w: basePath: %w", ErrInvalidPath, err)
	}
	ns, err := cleanPath(input.Namespace)
	if err != nil {
		return "", fmt.Errorf("%w: namespace: %w", ErrInvalidPath, err)
	}
	conn, err := cleanPath(input.ConnectionName)
	if err != nil {
		return "", fmt.Errorf("%w: connectionName: %w", ErrInvalidPath, err)
	}
	name, err := cleanPath(filename)
	if err != nil {
		return "", fmt.Errorf("%w: filename: %w", ErrInvalidPath, err)
	}
	return path.Join(base, ns, conn, name), nil
}

func cleanPath(value string) (string, error) {
	cleaned := path.Clean(strings.TrimSpace(value))
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") || path.IsAbs(cleaned) {
		return "", fmt.Errorf("%q escapes repository path", value)
	}
	return cleaned, nil
}
