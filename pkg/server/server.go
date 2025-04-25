package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/andrewweb/hackday/pkg/auth"
	"github.com/andrewweb/hackday/pkg/repo"
	"github.com/google/go-github/v45/github"
	"github.com/xanzy/go-gitlab"
)

type AnalysisRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Arguments   map[string]interface{} `json:"arguments"`
}

type AnalysisResponse struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Data    map[string]int `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type Server struct {
	port int
}

func NewServer(port int) *Server {
	return &Server{
		port: port,
	}
}

func (s *Server) Start() error {
	// Create a new mux to handle routes
	mux := http.NewServeMux()
	mux.HandleFunc("/messages", s.handleMessages)

	// Add logging middleware
	handler := loggingMiddleware(mux)

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("Starting server on port %d...\n", s.port)
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

	// Get blame information
	blameInfo, err := repoClient.GetBlameInfo(repository, pullRequest, selectedPR.ChangedFiles)
	if err != nil {
		sendErrorResponse(w, fmt.Sprintf("Failed to get blame information: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert blame info to a simpler map for JSON response
	blameData := make(map[string]int)
	for _, info := range blameInfo {
		blameData[info.User] = info.Lines
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AnalysisResponse{
		Status:  "success",
		Message: "Analysis completed",
		Data:    blameData,
	})
}

func sendErrorResponse(w http.ResponseWriter, errorMsg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(AnalysisResponse{
		Status: "error",
		Error:  errorMsg,
	})
}
