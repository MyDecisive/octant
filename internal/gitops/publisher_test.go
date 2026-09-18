package gitops

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-github/v75/github"
	"github.com/mydecisive/octant/internal/integration"
	integrationmock "github.com/mydecisive/octant/internal/mock/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeGitClient struct {
	refs      map[string]string
	treeBase  string
	entries   []*github.TreeEntry
	commit    github.Commit
	createRef github.CreateRef
	updateRef string
	updateSHA string
}

func newFakeGitClient(baseSHA string) *fakeGitClient {
	return &fakeGitClient{refs: map[string]string{"heads/main": baseSHA}}
}

func (c *fakeGitClient) GetRef(_ context.Context, _, _, ref string) (*github.Reference, *github.Response, error) {
	sha, ok := c.refs[ref]
	if !ok {
		return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, &github.ErrorResponse{
			Message: "Not Found",
		}
	}
	return &github.Reference{Object: &github.GitObject{SHA: github.Ptr(sha)}}, nil, nil
}

func (c *fakeGitClient) CreateRef(
	_ context.Context,
	_, _ string,
	ref github.CreateRef,
) (*github.Reference, *github.Response, error) {
	c.createRef = ref
	c.refs["heads/"+ref.Ref[len("refs/heads/"):]] = ref.SHA
	return &github.Reference{Object: &github.GitObject{SHA: github.Ptr(ref.SHA)}}, nil, nil
}

func (*fakeGitClient) GetCommit(_ context.Context, _, _, sha string) (*github.Commit, *github.Response, error) {
	return &github.Commit{
		SHA:  github.Ptr(sha),
		Tree: &github.Tree{SHA: github.Ptr("base-tree-sha")},
	}, nil, nil
}

func (c *fakeGitClient) CreateTree(
	_ context.Context,
	_, _, baseTree string,
	entries []*github.TreeEntry,
) (*github.Tree, *github.Response, error) {
	c.treeBase = baseTree
	c.entries = entries
	return &github.Tree{SHA: github.Ptr("tree-sha")}, nil, nil
}

func (c *fakeGitClient) CreateCommit(
	_ context.Context,
	_, _ string,
	commit github.Commit,
	_ *github.CreateCommitOptions,
) (*github.Commit, *github.Response, error) {
	c.commit = commit
	return &github.Commit{
		SHA:     github.Ptr("commit-sha"),
		HTMLURL: github.Ptr("https://github.example/commit/commit-sha"),
	}, nil, nil
}

func (c *fakeGitClient) UpdateRef(
	_ context.Context,
	_, _, ref string,
	updateRef github.UpdateRef,
) (*github.Reference, *github.Response, error) {
	c.updateRef = ref
	c.updateSHA = updateRef.SHA
	c.refs[ref] = updateRef.SHA
	return &github.Reference{Object: &github.GitObject{SHA: github.Ptr(updateRef.SHA)}}, nil, nil
}

type fakePRClient struct {
	existing []*github.PullRequest
	listOpts *github.PullRequestListOptions
	created  *github.NewPullRequest
}

func (c *fakePRClient) List(
	_ context.Context,
	_, _ string,
	opts *github.PullRequestListOptions,
) ([]*github.PullRequest, *github.Response, error) {
	c.listOpts = opts
	return c.existing, nil, nil
}

func (c *fakePRClient) Create(
	_ context.Context,
	_, _ string,
	pull *github.NewPullRequest,
) (*github.PullRequest, *github.Response, error) {
	c.created = pull
	return &github.PullRequest{
		Number:  github.Ptr(7),
		HTMLURL: github.Ptr("https://github.example/pulls/7"),
	}, nil, nil
}

func testCreds() integration.GitHubAppIntegrationData {
	return integration.GitHubAppIntegrationData{
		AppID:          123,
		InstallationID: 456,
		PrivateKey:     "private-key",
		Owner:          "mydecisive",
		Repository:     "ci",
		BaseBranch:     "main",
		BranchPrefix:   "octant/manifests",
		BasePath:       "clusters/dev",
		CommitterName:  "octant",
		CommitterEmail: "octant@example.com",
	}
}

func newTestPublisher(
	t *testing.T,
	creds *integration.GitHubAppIntegrationData,
	git *fakeGitClient,
	prs *fakePRClient,
) *GitHubManifestPublisher {
	t.Helper()

	mockIntegration := integrationmock.NewMockIntegration[integration.GitHubAppIntegrationData](t)
	mockIntegration.EXPECT().GetIntegrationByName(mock.Anything, defaultIntegrationName).Return(creds, nil).Maybe()

	publisher := NewGitHubManifestPublisher(mockIntegration)
	publisher.newClients = func(integration.GitHubAppIntegrationData) (installationClients, error) {
		return installationClients{git: git, pr: prs}, nil
	}
	return publisher
}

func TestGitHubManifestPublisherPublishManifests(t *testing.T) {
	t.Parallel()

	creds := testCreds()
	git := newFakeGitClient("base-sha")
	prs := &fakePRClient{}
	publisher := newTestPublisher(t, &creds, git, prs)

	result, err := publisher.PublishManifests(t.Context(), PublishInput{
		ConnectionName: "orders",
		Namespace:      "mdai",
		MDAIVersion:    "0.9.0",
		Manifests: map[string][]byte{
			"connection.yaml": []byte("kind: ConfigMap\n"),
			"validator.yaml":  []byte("kind: Job\n"),
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "base-tree-sha", git.treeBase, "CreateTree must use the base commit's tree SHA, not its commit SHA")
	require.Len(t, git.entries, 2)
	assert.ElementsMatch(t, []string{
		"clusters/dev/mdai/orders/connection.yaml",
		"clusters/dev/mdai/orders/validator.yaml",
	}, []string{git.entries[0].GetPath(), git.entries[1].GetPath()})
	assert.Equal(t, "chore: update orders manifests", git.commit.GetMessage())
	assert.Equal(t, "tree-sha", git.commit.GetTree().GetSHA())
	require.Len(t, git.commit.Parents, 1)
	assert.Equal(t, "base-sha", git.commit.Parents[0].GetSHA())

	assert.Equal(t, "refs/heads/octant/manifests/mdai-orders", git.createRef.Ref)
	assert.Equal(t, "commit-sha", git.createRef.SHA)

	require.NotNil(t, prs.created)
	assert.Equal(t, "octant/manifests/mdai-orders", prs.created.GetHead())
	assert.Equal(t, "main", prs.created.GetBase())
	assert.Equal(t, "chore: update orders manifests", prs.created.GetTitle())

	assert.Equal(t, "octant/manifests/mdai-orders", result.Branch)
	assert.Equal(t, "commit-sha", result.CommitSHA)
	assert.Equal(t, 7, result.PullRequestNumber)
	assert.Equal(t, "https://github.example/pulls/7", result.PullRequestURL)
	assert.Equal(t, 2, result.Files)
}

func TestGitHubManifestPublisherReusesExistingOpenPullRequest(t *testing.T) {
	t.Parallel()

	creds := testCreds()
	git := newFakeGitClient("base-sha")
	git.refs["heads/octant/manifests/mdai-orders"] = "old-commit-sha"
	prs := &fakePRClient{existing: []*github.PullRequest{{
		Number:  github.Ptr(3),
		HTMLURL: github.Ptr("https://github.example/pulls/3"),
	}}}
	publisher := newTestPublisher(t, &creds, git, prs)

	result, err := publisher.PublishManifests(t.Context(), PublishInput{
		ConnectionName: "orders",
		Namespace:      "mdai",
		Manifests: map[string][]byte{
			"connection.yaml": []byte("kind: ConfigMap\n"),
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "heads/octant/manifests/mdai-orders", git.updateRef, "existing branch should be updated in place")
	assert.Equal(t, "commit-sha", git.updateSHA)
	assert.Nil(t, prs.created, "should not open a duplicate pull request")
	assert.Equal(t, 3, result.PullRequestNumber)
}

func TestGitHubManifestPublisherRejectsEscapingPath(t *testing.T) {
	t.Parallel()

	creds := testCreds()
	publisher := newTestPublisher(t, &creds, newFakeGitClient("base-sha"), &fakePRClient{})

	_, err := publisher.PublishManifests(t.Context(), PublishInput{
		ConnectionName: "../orders",
		Namespace:      "mdai",
		Manifests: map[string][]byte{
			"connection.yaml": []byte("kind: ConfigMap\n"),
		},
	})

	require.ErrorIs(t, err, ErrInvalidPath)
}

func TestGitHubManifestPublisherNoIntegrationConfigured(t *testing.T) {
	t.Parallel()

	publisher := newTestPublisher(t, nil, newFakeGitClient("base-sha"), &fakePRClient{})

	result, err := publisher.PublishManifests(t.Context(), PublishInput{
		ConnectionName: "orders",
		Namespace:      "mdai",
		Manifests: map[string][]byte{
			"connection.yaml": []byte("kind: ConfigMap\n"),
		},
	})

	require.NoError(t, err)
	assert.Nil(t, result)
}

type fakeRepoClient struct {
	owner, repo string
	err         error
}

func (c *fakeRepoClient) Get(_ context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
	c.owner, c.repo = owner, repo
	if c.err != nil {
		return nil, nil, c.err
	}
	return &github.Repository{}, nil, nil
}

func TestGitHubManifestPublisherTestConnection(t *testing.T) {
	t.Parallel()

	repo := &fakeRepoClient{}
	publisher := NewGitHubManifestPublisher(nil)
	publisher.newClients = func(integration.GitHubAppIntegrationData) (installationClients, error) {
		return installationClients{repo: repo}, nil
	}

	ok, err := publisher.TestConnection(t.Context(), integration.GitHubAppIntegrationData{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     "key",
		Owner:          "mydecisive",
		Repository:     "ci",
	})

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "mydecisive", repo.owner)
	assert.Equal(t, "ci", repo.repo)
}
