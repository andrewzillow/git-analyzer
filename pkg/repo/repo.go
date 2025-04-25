package repo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/xanzy/go-gitlab"
)

type Repository struct {
	Name     string
	FullName string
	URL      string
	Provider string
}

func ListGitHubRepos(client *github.Client) ([]Repository, error) {
	ctx := context.Background()
	repos, _, err := client.Repositories.List(ctx, "", &github.RepositoryListOptions{
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list GitHub repositories: %v", err)
	}

	// Sort repositories alphabetically by full name
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].GetFullName() < repos[j].GetFullName()
	})

	var result []Repository
	for _, repo := range repos {
		result = append(result, Repository{
			Name:     repo.GetName(),
			FullName: repo.GetFullName(),
			URL:      repo.GetHTMLURL(),
			Provider: "github",
		})
	}

	return result, nil
}

func ListGitLabRepos(client *gitlab.Client) ([]Repository, error) {
	opt := &gitlab.ListProjectsOptions{
		OrderBy: gitlab.String("updated_at"),
		Sort:    gitlab.String("desc"),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	projects, _, err := client.Projects.ListProjects(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list GitLab repositories: %v", err)
	}

	var result []Repository
	for _, project := range projects {
		result = append(result, Repository{
			Name:     project.Name,
			FullName: project.PathWithNamespace,
			URL:      project.WebURL,
			Provider: "gitlab",
		})
	}

	return result, nil
}

func FormatRepoList(repos []Repository) string {
	var sb strings.Builder
	sb.WriteString("\nAvailable Repositories:\n")
	sb.WriteString("----------------------\n")
	for i, repo := range repos {
		sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, repo.FullName, repo.Provider))
	}
	return sb.String()
}
