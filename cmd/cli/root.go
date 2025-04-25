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
		token = auth.GetTokenFromEnv(provider)
		if token == "" {
			fmt.Printf("Enter %s personal access token: ", provider)
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %v", err)
			}
			token = strings.TrimSpace(input)
		}
	}

	// Authenticate
	var authProvider auth.AuthProvider
	switch provider {
	case "github":
		authProvider = auth.NewGitHubAuth(token)
	case "gitlab":
		authProvider = auth.NewGitLabAuth(token)
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	if err := authProvider.Authenticate(); err != nil {
		return err
	}
	fmt.Printf("Successfully authenticated with %s\n", provider)

	// List repositories
	var repos []repo.Repository
	var err error
	switch provider {
	case "github":
		repos, err = repo.ListGitHubRepos(authProvider.GetClient().(*github.Client))
	case "gitlab":
		repos, err = repo.ListGitLabRepos(authProvider.GetClient().(*gitlab.Client))
	}
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

	return nil
}

var rootCmd = &cobra.Command{
	Use:   "repo-analyzer",
	Short: "A tool to analyze GitHub and GitLab repositories",
	Long:  `A CLI tool that allows authentication to GitHub or GitLab and repository analysis.`,
	RunE:  executeRoot,
}
