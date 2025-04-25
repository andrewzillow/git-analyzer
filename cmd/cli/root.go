package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/andrewweb/hackday/pkg/auth"
	"github.com/andrewweb/hackday/pkg/repo"
	"github.com/google/go-github/v45/github"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	provider string
	token    string
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&provider, "provider", "p", "", "Git provider (github or gitlab)")
	rootCmd.PersistentFlags().StringVarP(&token, "token", "t", "", "Personal access token")
}

func executeRoot(cmd *cobra.Command, args []string) error {
	// Get provider if not specified
	if provider == "" {
		fmt.Print("Select provider (github/gitlab): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %v", err)
		}
		provider = strings.TrimSpace(strings.ToLower(input))
	}

	// Get token if not specified
	if token == "" {
		// Try to get token from environment first
		token = auth.GetTokenFromEnv(provider)

		// If not in environment, try to get from cache
		if token == "" {
			cachedToken, err := auth.GetCachedToken(provider)
			if err != nil {
				fmt.Printf("Warning: Failed to load cached token: %v\n", err)
			}
			token = cachedToken
		}

		// If still no token, prompt user
		if token == "" {
			fmt.Printf("Enter %s personal access token: ", provider)
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %v", err)
			}
			token = strings.TrimSpace(input)

			// Save the token to cache
			if err := auth.SaveToken(provider, token); err != nil {
				fmt.Printf("Warning: Failed to save token to cache: %v\n", err)
			}
		}
	}

	// Create repository client based on provider
	var repoClient repo.RepositoryClient
	switch provider {
	case "github":
		authProvider := auth.NewGitHubAuth(token)
		if err := authProvider.Authenticate(); err != nil {
			return err
		}
		repoClient = repo.NewGitHubClient(authProvider.GetClient().(*github.Client))
	case "gitlab":
		authProvider := auth.NewGitLabAuth(token)
		if err := authProvider.Authenticate(); err != nil {
			return err
		}
		repoClient = repo.NewGitLabClient(authProvider.GetClient().(*gitlab.Client))
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	fmt.Printf("Successfully authenticated with %s\n", provider)

	// List repositories
	repos, err := repoClient.ListRepositories()
	if err != nil {
		return err
	}

	// Display repositories and get selection
	fmt.Println(repo.FormatRepoList(repos))
	fmt.Print("Select a repository (number): ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %v", err)
	}

	selection, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || selection < 1 || selection > len(repos) {
		return fmt.Errorf("invalid selection")
	}

	selectedRepo := repos[selection-1]
	fmt.Printf("\nSelected repository: %s\n", selectedRepo.FullName)
	fmt.Printf("URL: %s\n", selectedRepo.URL)

	// List pull requests
	prs, err := repoClient.ListPullRequests(selectedRepo.FullName)
	if err != nil {
		return err
	}

	if len(prs) == 0 {
		fmt.Println("\nNo open pull requests found.")
		return nil
	}

	// Display pull requests and get selection
	fmt.Println(repo.FormatPullRequestList(prs))
	fmt.Print("Select a pull request (number): ")
	input, err = reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %v", err)
	}

	selection, err = strconv.Atoi(strings.TrimSpace(input))
	if err != nil || selection < 1 || selection > len(prs) {
		return fmt.Errorf("invalid selection")
	}

	selectedPR := prs[selection-1]
	fmt.Printf("\nSelected pull request: #%d - %s\n", selectedPR.Number, selectedPR.Title)
	fmt.Printf("URL: %s\n", selectedPR.URL)

	// Display changed files
	fmt.Println(repo.FormatChangedFiles(selectedPR.ChangedFiles))

	// Get blame information
	blameInfo, err := repoClient.GetBlameInfo(selectedRepo.FullName, selectedPR.Number, selectedPR.ChangedFiles)
	if err != nil {
		return err
	}

	// Display blame information
	fmt.Println(repo.FormatBlameInfo(blameInfo))

	return nil
}

var rootCmd = &cobra.Command{
	Use:   "repo-analyzer",
	Short: "A tool to analyze GitHub and GitLab repositories",
	Long:  `A CLI tool that allows authentication to GitHub or GitLab and repository analysis.`,
	RunE:  executeRoot,
}
