package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

	// Add subcommands
	rootCmd.AddCommand(blameCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(serverCmd)
}

var blameCmd = &cobra.Command{
	Use:   "blame",
	Short: "Run git-blame analysis on a repository",
	Long:  `Analyzes the blame information for files in a pull request, showing which authors modified which lines.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAnalysis("blame")
	},
}

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Run git-log analysis on a repository",
	Long:  `Runs code-maat analysis on the repository's git log.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAnalysis("log")
	},
}

func splitRepoFullName(fullName string) (string, string, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repository name format: %s. Expected format: owner/repo", fullName)
	}
	return parts[0], parts[1], nil
}

func runAnalysis(analysisType string) error {
	// Get analysis type if not specified
	if analysisType == "" {
		fmt.Print("Select analysis type (blame/log): ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %v", err)
		}
		analysisType = strings.TrimSpace(strings.ToLower(input))
		if analysisType != "blame" && analysisType != "log" {
			return fmt.Errorf("invalid analysis type. Must be 'blame' or 'log'")
		}
	}

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

	// Run the selected analysis
	switch analysisType {
	case "blame":
		// Get blame information
		blameInfo, err := repoClient.GetBlameInfo(selectedRepo.FullName, selectedPR.Number, selectedPR.ChangedFiles)
		if err != nil {
			return err
		}

		// Display blame information
		fmt.Println(repo.FormatBlameInfo(blameInfo))

	case "log":
		// Get commit history using GitHub API
		ctx := context.Background()
		owner, repoName, err := splitRepoFullName(selectedRepo.FullName)
		if err != nil {
			return fmt.Errorf("failed to parse repository name: %v", err)
		}

		// Get all commits for the repository
		commits, err := repoClient.(*repo.GitHubClient).GetCommits(ctx, owner, repoName, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
		if err != nil {
			return fmt.Errorf("failed to get commits: %v", err)
		}

		// Create a temporary directory for the log file
		tempDir, err := os.MkdirTemp("", "git-log-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Format the commits in the required format for code-maat
		var logContent strings.Builder
		for _, commit := range commits {
			// Get the commit details to get the changed files
			commitDetails, err := repoClient.(*repo.GitHubClient).GetCommitDetails(ctx, owner, repoName, commit.GetSHA())
			if err != nil {
				return fmt.Errorf("failed to get commit details: %v", err)
			}

			// Write the commit header
			logContent.WriteString(fmt.Sprintf("--%s--%s--%s\n",
				commit.GetSHA()[:7],
				commit.GetCommit().GetCommitter().GetDate().Format("2006-01-02"),
				commit.GetCommit().GetCommitter().GetName()))

			// Write the changed files
			for _, file := range commitDetails.Files {
				logContent.WriteString(fmt.Sprintf("%d\t%d\t%s\n",
					file.GetAdditions(),
					file.GetDeletions(),
					file.GetFilename()))
			}
		}

		// Write the log content to a file
		logFile := filepath.Join(tempDir, "logfile.log")
		if err := os.WriteFile(logFile, []byte(logContent.String()), 0644); err != nil {
			return fmt.Errorf("failed to write log file: %v", err)
		}

		// Print the log file content for debugging
		fmt.Println("\nLog file content:")
		fmt.Println(logContent.String())

		// Check if code-maat jar exists
		jarPath := "code-maat-1.0.4-standalone.jar"
		if _, err := os.Stat(jarPath); os.IsNotExist(err) {
			return fmt.Errorf("code-maat jar file not found. Please download it from https://github.com/adamtornhill/code-maat/releases and place it in the current directory")
		}

		// First try a simpler analysis
		fmt.Println("\nTrying simple analysis first...")
		simpleCmd := exec.Command("java", "-jar", jarPath, "-l", logFile, "-c", "git2", "-a", "summary")
		var stderr bytes.Buffer
		simpleCmd.Stderr = &stderr
		simpleOutput, err := simpleCmd.Output()
		if err != nil {
			fmt.Printf("Simple analysis failed: %v\nError output: %s\n", err, stderr.String())
		} else {
			fmt.Println("Simple analysis succeeded:")
			fmt.Println(string(simpleOutput))
		}

		// Now try the fragmentation analysis
		fmt.Println("\nTrying fragmentation analysis...")
		codeMaatCmd := exec.Command("java", "-jar", jarPath, "-l", logFile, "-c", "git2", "-a", "fragmentation")
		codeMaatCmd.Stderr = &stderr
		codeMaatOutput, err := codeMaatCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to run code-maat: %v\nError output: %s", err, stderr.String())
		}

		// Display the analysis results
		fmt.Println("\nCode Maat Analysis Results:")
		fmt.Println(string(codeMaatOutput))
	}

	return nil
}

func executeRoot(cmd *cobra.Command, args []string) error {
	return runAnalysis("")
}

var rootCmd = &cobra.Command{
	Use:   "repo-analyzer",
	Short: "A tool to analyze GitHub and GitLab repositories",
	Long:  `A CLI tool that allows authentication to GitHub or GitLab and repository analysis.`,
	RunE:  executeRoot,
}
