package auth

import (
	"fmt"
	"os"

	"github.com/google/go-github/v45/github"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
)

type AuthProvider interface {
	Authenticate() error
	GetClient() interface{}
}

type GitHubAuth struct {
	client *github.Client
	token  string
}

type GitLabAuth struct {
	client *gitlab.Client
	token  string
}

func NewGitHubAuth(token string) *GitHubAuth {
	return &GitHubAuth{
		token: token,
	}
}

func NewGitLabAuth(token string) *GitLabAuth {
	return &GitLabAuth{
		token: token,
	}
}

func (g *GitHubAuth) Authenticate() error {
	ctx := oauth2.NoContext
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: g.token},
	)
	tc := oauth2.NewClient(ctx, ts)
	g.client = github.NewClient(tc)

	// Verify the token works
	_, _, err := g.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to authenticate with GitHub: %v", err)
	}

	return nil
}

func (g *GitHubAuth) GetClient() interface{} {
	return g.client
}

func (g *GitLabAuth) Authenticate() error {
	client, err := gitlab.NewClient(g.token)
	if err != nil {
		return fmt.Errorf("failed to create GitLab client: %v", err)
	}
	g.client = client

	// Verify the token works
	_, _, err = g.client.Users.CurrentUser()
	if err != nil {
		return fmt.Errorf("failed to authenticate with GitLab: %v", err)
	}

	return nil
}

func (g *GitLabAuth) GetClient() interface{} {
	return g.client
}

func GetTokenFromEnv(provider string) string {
	switch provider {
	case "github":
		return os.Getenv("GITHUB_TOKEN")
	case "gitlab":
		return os.Getenv("GITLAB_TOKEN")
	default:
		return ""
	}
}
