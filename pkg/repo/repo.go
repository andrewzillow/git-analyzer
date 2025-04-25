package repo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/xanzy/go-gitlab"
)

// RepositoryClient defines the interface for repository operations
type RepositoryClient interface {
	ListRepositories() ([]Repository, error)
	ListPullRequests(repoFullName string) ([]PullRequest, error)
	GetBlameInfo(repoFullName string, prNumber int, files []string) (map[string]BlameInfo, error)
}

type Repository struct {
	Name     string
	FullName string
	URL      string
	Provider string
}

type PullRequest struct {
	Number       int
	Title        string
	State        string
	URL          string
	Provider     string
	ChangedFiles []string
}

type BlameInfo struct {
	User  string
	Lines int
}

// GitHubClient implements RepositoryClient for GitHub
type GitHubClient struct {
	client *github.Client
}

func NewGitHubClient(client *github.Client) *GitHubClient {
	return &GitHubClient{client: client}
}

func (c *GitHubClient) ListRepositories() ([]Repository, error) {
	ctx := context.Background()
	repos, _, err := c.client.Repositories.List(ctx, "", &github.RepositoryListOptions{
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

func (c *GitHubClient) ListPullRequests(repoFullName string) ([]PullRequest, error) {
	ctx := context.Background()
	owner, repo := splitRepoFullName(repoFullName)

	prs, _, err := c.client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %v", err)
	}

	var result []PullRequest
	for _, pr := range prs {
		// Get changed files for each PR
		files, _, err := c.client.PullRequests.ListFiles(ctx, owner, repo, pr.GetNumber(), &github.ListOptions{PerPage: 100})
		if err != nil {
			return nil, fmt.Errorf("failed to get changed files: %v", err)
		}

		var changedFiles []string
		for _, file := range files {
			changedFiles = append(changedFiles, file.GetFilename())
		}

		result = append(result, PullRequest{
			Number:       pr.GetNumber(),
			Title:        pr.GetTitle(),
			State:        pr.GetState(),
			URL:          pr.GetHTMLURL(),
			Provider:     "github",
			ChangedFiles: changedFiles,
		})
	}

	return result, nil
}

func (c *GitHubClient) GetBlameInfo(repoFullName string, prNumber int, files []string) (map[string]BlameInfo, error) {
	ctx := context.Background()
	owner, repo := splitRepoFullName(repoFullName)

	blameInfo := make(map[string]BlameInfo)

	for _, filename := range files {
		// Get the file's commit history
		commits, _, err := c.client.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
			Path: filename,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get commits for file %s: %v", filename, err)
		}

		// For each commit, count the number of lines it modified
		for _, commit := range commits {
			// Try to get author name in order of preference
			var author string
			if commit.GetAuthor() != nil {
				author = commit.GetAuthor().GetLogin()
				if author == "" {
					author = commit.GetAuthor().GetName()
				}
			}
			if author == "" && commit.GetCommitter() != nil {
				author = commit.GetCommitter().GetLogin()
				if author == "" {
					author = commit.GetCommitter().GetName()
				}
			}
			if author == "" {
				author = "Unknown Author"
			}

			// Get the commit details to see what files were modified
			commitDetails, _, err := c.client.Repositories.GetCommit(ctx, owner, repo, commit.GetSHA(), nil)
			if err != nil {
				return nil, fmt.Errorf("failed to get commit details: %v", err)
			}

			// Count lines modified in this commit for this file
			for _, file := range commitDetails.Files {
				if file.GetFilename() == filename {
					info := blameInfo[author]
					info.User = author
					info.Lines += file.GetChanges()
					blameInfo[author] = info
					break
				}
			}
		}
	}

	return blameInfo, nil
}

// GitLabClient implements RepositoryClient for GitLab
type GitLabClient struct {
	client *gitlab.Client
}

func NewGitLabClient(client *gitlab.Client) *GitLabClient {
	return &GitLabClient{client: client}
}

func (c *GitLabClient) ListRepositories() ([]Repository, error) {
	opt := &gitlab.ListProjectsOptions{
		OrderBy: gitlab.String("updated_at"),
		Sort:    gitlab.String("desc"),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	projects, _, err := c.client.Projects.ListProjects(opt)
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

func (c *GitLabClient) ListPullRequests(repoFullName string) ([]PullRequest, error) {
	opt := &gitlab.ListProjectMergeRequestsOptions{
		State: gitlab.String("opened"),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	mrs, _, err := c.client.MergeRequests.ListProjectMergeRequests(repoFullName, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list merge requests: %v", err)
	}

	var result []PullRequest
	for _, mr := range mrs {
		// Get changed files for each MR
		changes, _, err := c.client.MergeRequests.GetMergeRequestChanges(repoFullName, mr.IID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get changed files: %v", err)
		}

		var changedFiles []string
		for _, change := range changes.Changes {
			changedFiles = append(changedFiles, change.NewPath)
		}

		result = append(result, PullRequest{
			Number:       mr.IID,
			Title:        mr.Title,
			State:        mr.State,
			URL:          mr.WebURL,
			Provider:     "gitlab",
			ChangedFiles: changedFiles,
		})
	}

	return result, nil
}

func (c *GitLabClient) GetBlameInfo(repoFullName string, prNumber int, files []string) (map[string]BlameInfo, error) {
	blameInfo := make(map[string]BlameInfo)

	for _, filename := range files {
		// Get the file's commit history
		commits, _, err := c.client.Commits.ListCommits(repoFullName, &gitlab.ListCommitsOptions{
			Path: gitlab.String(filename),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get commits for file %s: %v", filename, err)
		}

		// For each commit, count the number of lines it modified
		for _, commit := range commits {
			author := commit.AuthorName
			if author == "" {
				author = commit.AuthorEmail
			}

			// Get the diff for this commit
			diffs, _, err := c.client.Commits.GetCommitDiff(repoFullName, commit.ID, &gitlab.GetCommitDiffOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get commit diff: %v", err)
			}

			// Count lines modified in this commit for this file
			for _, diff := range diffs {
				if diff.NewPath == filename {
					info := blameInfo[author]
					info.User = author
					// Count the number of lines in the diff
					lines := strings.Split(diff.Diff, "\n")
					info.Lines += len(lines)
					blameInfo[author] = info
					break
				}
			}
		}
	}

	return blameInfo, nil
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

func FormatPullRequestList(prs []PullRequest) string {
	var sb strings.Builder
	sb.WriteString("\nOpen Pull Requests:\n")
	sb.WriteString("------------------\n")
	for i, pr := range prs {
		sb.WriteString(fmt.Sprintf("%d. #%d: %s\n", i+1, pr.Number, pr.Title))
	}
	return sb.String()
}

func FormatChangedFiles(files []string) string {
	var sb strings.Builder
	sb.WriteString("\nChanged Files:\n")
	sb.WriteString("--------------\n")
	for _, file := range files {
		sb.WriteString(fmt.Sprintf("- %s\n", file))
	}
	return sb.String()
}

func FormatBlameInfo(blameInfo map[string]BlameInfo) string {
	var sb strings.Builder
	sb.WriteString("\nAuthors and Lines Touched:\n")
	sb.WriteString("-------------------------\n")

	// Convert map to slice for sorting
	var infoSlice []BlameInfo
	for _, info := range blameInfo {
		infoSlice = append(infoSlice, info)
	}

	// Sort by number of lines (descending)
	sort.Slice(infoSlice, func(i, j int) bool {
		return infoSlice[i].Lines > infoSlice[j].Lines
	})

	for _, info := range infoSlice {
		sb.WriteString(fmt.Sprintf("%s: %d lines\n", info.User, info.Lines))
	}

	return sb.String()
}

func splitRepoFullName(fullName string) (string, string) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
