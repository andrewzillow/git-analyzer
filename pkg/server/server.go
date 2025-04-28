package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/andrewweb/hackday/pkg/auth"
	"github.com/andrewweb/hackday/pkg/repo"
	"github.com/google/go-github/v45/github"
	"github.com/xanzy/go-gitlab"
)

type AnalysisRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type AnalysisResponse struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Data    map[string]string `json:"data,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type Argument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type Prompt struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Arguments   []Argument `json:"arguments"`
}

type Server struct {
	port int
	mux  *http.ServeMux
}

func NewServer(port int) *Server {
	mux := http.NewServeMux()
	server := &Server{
		port: port,
		mux:  mux,
	}

	// Register routes
	mux.HandleFunc("/messages", server.handleMessages)
	mux.HandleFunc("/prompts", server.handlePrompts)

	return server
}

func (s *Server) Start() error {
	// Add logging middleware
	handler := loggingMiddleware(s.mux)

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Starting server on port %d...\n", s.port)
	fmt.Printf("Registered routes:\n")
	fmt.Printf("- GET /prompts\n")
	fmt.Printf("- POST /messages\n")
	return http.ListenAndServe(addr, handler)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request for %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validate(w http.ResponseWriter, r *http.Request) (*AnalysisRequest, error) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		log.Printf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil, fmt.Errorf("method not allowed")
	}

	// Ensure content type is application/json
	if r.Header.Get("Content-Type") != "application/json" {
		log.Printf("Invalid content type: %s", r.Header.Get("Content-Type"))
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return nil, fmt.Errorf("invalid content type")
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return nil, fmt.Errorf("error reading request body")
	}
	defer r.Body.Close()

	// Parse the request body
	var req AnalysisRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return nil, fmt.Errorf("invalid JSON format")
	}

	// Validate request
	if req.Name != "git-blame" && req.Name != "git-log" {
		sendErrorResponse(w, "Invalid name. Must be one of: 'git-blame', 'git-log'", http.StatusBadRequest)
		return nil, fmt.Errorf("invalid name")
	}

	// Extract and validate arguments
	var providerType repo.ProviderType
	var token string
	var repository string
	var pullRequest int

	if providerVal, ok := req.Arguments["provider"]; ok {
		if str, ok := providerVal.(string); ok {
			providerType = repo.ProviderType(str)
			if !providerType.IsValid() {
				sendErrorResponse(w, fmt.Sprintf("Invalid provider type. Must be one of: %s, %s", repo.GitHub, repo.GitLab), http.StatusBadRequest)
				return nil, fmt.Errorf("invalid provider type")
			}
		} else {
			sendErrorResponse(w, "Provider type must be a string", http.StatusBadRequest)
			return nil, fmt.Errorf("invalid provider type format")
		}
	} else {
		sendErrorResponse(w, "Provider is required", http.StatusBadRequest)
		return nil, fmt.Errorf("provider is required")
	}

	if tokenVal, ok := req.Arguments["token"]; ok {
		if str, ok := tokenVal.(string); ok {
			token = str
		} else {
			sendErrorResponse(w, "Token must be a string", http.StatusBadRequest)
			return nil, fmt.Errorf("invalid token format")
		}
	} else {
		sendErrorResponse(w, "Token is required", http.StatusBadRequest)
		return nil, fmt.Errorf("token is required")
	}

	if repoVal, ok := req.Arguments["repository"]; ok {
		if str, ok := repoVal.(string); ok {
			repository = str
		} else {
			sendErrorResponse(w, "Repository must be a string", http.StatusBadRequest)
			return nil, fmt.Errorf("invalid repository format")
		}
	} else {
		sendErrorResponse(w, "Repository is required", http.StatusBadRequest)
		return nil, fmt.Errorf("repository is required")
	}

	if prVal, ok := req.Arguments["pullRequest"]; ok {
		if num, ok := prVal.(float64); ok {
			pullRequest = int(num)
		} else {
			sendErrorResponse(w, "Pull request must be a number", http.StatusBadRequest)
			return nil, fmt.Errorf("invalid pull request format")
		}
	} else {
		sendErrorResponse(w, "Pull request is required", http.StatusBadRequest)
		return nil, fmt.Errorf("pull request is required")
	}

	// Validate required arguments
	if token == "" {
		sendErrorResponse(w, "Token is required", http.StatusBadRequest)
		return nil, fmt.Errorf("token is required")
	}
	if repository == "" {
		sendErrorResponse(w, "Repository is required", http.StatusBadRequest)
		return nil, fmt.Errorf("repository is required")
	}
	if pullRequest <= 0 {
		sendErrorResponse(w, "Pull request number must be positive", http.StatusBadRequest)
		return nil, fmt.Errorf("pull request number must be positive")
	}

	// Create a new request with the extracted values
	req.Arguments = map[string]interface{}{
		"provider":    providerType,
		"token":       token,
		"repository":  repository,
		"pullRequest": pullRequest,
	}

	return &req, nil
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	// Validate request and parse body
	req, err := s.validate(w, r)
	if err != nil {
		return
	}

	// Extract arguments
	providerType := req.Arguments["provider"].(repo.ProviderType)
	token := req.Arguments["token"].(string)
	repository := req.Arguments["repository"].(string)
	pullRequest := req.Arguments["pullRequest"].(int)

	// Create repository client based on provider
	var repoClient repo.RepositoryClient
	switch providerType {
	case repo.GitHub:
		authProvider := auth.NewGitHubAuth(token)
		if err := authProvider.Authenticate(); err != nil {
			sendErrorResponse(w, fmt.Sprintf("GitHub authentication failed: %v", err), http.StatusUnauthorized)
			return
		}
		repoClient = repo.NewGitHubClient(authProvider.GetClient().(*github.Client))
	case repo.GitLab:
		authProvider := auth.NewGitLabAuth(token)
		if err := authProvider.Authenticate(); err != nil {
			sendErrorResponse(w, fmt.Sprintf("GitLab authentication failed: %v", err), http.StatusUnauthorized)
			return
		}
		repoClient = repo.NewGitLabClient(authProvider.GetClient().(*gitlab.Client))
	}

	// Get pull request information
	prs, err := repoClient.ListPullRequests(repository)
	if err != nil {
		sendErrorResponse(w, fmt.Sprintf("Failed to get pull requests: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the specific pull request
	var selectedPR *repo.PullRequest
	for _, pr := range prs {
		if pr.Number == pullRequest {
			selectedPR = &pr
			break
		}
	}

	if selectedPR == nil {
		sendErrorResponse(w, fmt.Sprintf("Pull request #%d not found", pullRequest), http.StatusNotFound)
		return
	}

	// Handle different message types
	switch req.Name {
	case "git-blame":
		// Get blame information
		blameInfo, err := repoClient.GetBlameInfo(repository, pullRequest, selectedPR.ChangedFiles)
		if err != nil {
			sendErrorResponse(w, fmt.Sprintf("Failed to get blame information: %v", err), http.StatusInternalServerError)
			return
		}

		// Convert blame info to a simpler map for JSON response
		blameData := make(map[string]string)
		for _, info := range blameInfo {
			blameData[info.User] = fmt.Sprintf("%d", info.Lines)
		}

		// Return success response with blame data
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AnalysisResponse{
			Status:  "success",
			Message: "Blame analysis completed",
			Data:    blameData,
		})

	case "git-log":
		// Create a temporary directory for the log files
		tempDir, err := os.MkdirTemp("", "git-log-*")
		if err != nil {
			sendErrorResponse(w, fmt.Sprintf("Failed to create temporary directory: %v", err), http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(tempDir)

		// Clone the repository
		cloneCmd := exec.Command("git", "clone", selectedPR.URL, tempDir)
		if err := cloneCmd.Run(); err != nil {
			sendErrorResponse(w, fmt.Sprintf("Failed to clone repository: %v", err), http.StatusInternalServerError)
			return
		}

		// Change to the repository directory
		if err := os.Chdir(tempDir); err != nil {
			sendErrorResponse(w, fmt.Sprintf("Failed to change directory: %v", err), http.StatusInternalServerError)
			return
		}

		// Run git log command
		logFile := filepath.Join(tempDir, "logfile.log")
		gitLogCmd := exec.Command("git", "log", "--all", "--numstat", "--date=short", "--pretty=format:--%h--%ad--%aN", "--no-renames", "--after=2024-01-01")
		output, err := gitLogCmd.Output()
		if err != nil {
			sendErrorResponse(w, fmt.Sprintf("Failed to run git log: %v", err), http.StatusInternalServerError)
			return
		}

		// Write git log output to file
		if err := os.WriteFile(logFile, output, 0644); err != nil {
			sendErrorResponse(w, fmt.Sprintf("Failed to write log file: %v", err), http.StatusInternalServerError)
			return
		}

		// Run code-maat
		codeMaatCmd := exec.Command("java", "-jar", "code-maat-1.0.4-standalone.jar", "-l", logFile, "-c", "git2", "-a", "fragmentation")
		codeMaatOutput, err := codeMaatCmd.Output()
		if err != nil {
			sendErrorResponse(w, fmt.Sprintf("Failed to run code-maat: %v", err), http.StatusInternalServerError)
			return
		}

		// Parse CSV output
		lines := strings.Split(string(codeMaatOutput), "\n")
		csvData := make(map[string]string)
		for i, line := range lines {
			if i == 0 {
				csvData["header"] = line
			} else if line != "" {
				csvData[fmt.Sprintf("row_%d", i)] = line
			}
		}

		// Return success response with CSV data
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AnalysisResponse{
			Status:  "success",
			Message: "Git log analysis completed",
			Data:    csvData,
		})
	}
}

func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		log.Printf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prompts := []Prompt{
		{
			Name:        "git-blame",
			Description: "Analyzes the blame information for files in a pull request, showing which authors modified which lines.",
			Arguments: []Argument{
				{
					Name:        "provider",
					Description: "The Git provider (github or gitlab)",
					Required:    true,
				},
				{
					Name:        "token",
					Description: "Personal access token for authentication",
					Required:    true,
				},
				{
					Name:        "repository",
					Description: "Full repository name in the format owner/repo",
					Required:    true,
				},
				{
					Name:        "pullRequest",
					Description: "Pull request number",
					Required:    true,
				},
			},
		},
		{
			Name:        "git-log",
			Description: "Returns a success response for the specified repository and pull request.",
			Arguments: []Argument{
				{
					Name:        "provider",
					Description: "The Git provider (github or gitlab)",
					Required:    true,
				},
				{
					Name:        "token",
					Description: "Personal access token for authentication",
					Required:    true,
				},
				{
					Name:        "repository",
					Description: "Full repository name in the format owner/repo",
					Required:    true,
				},
				{
					Name:        "pullRequest",
					Description: "Pull request number",
					Required:    true,
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(prompts)
}

func sendErrorResponse(w http.ResponseWriter, errorMsg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(AnalysisResponse{
		Status: "error",
		Error:  errorMsg,
	})
}
