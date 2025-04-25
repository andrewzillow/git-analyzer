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
	Type        string `json:"type"`
	Token       string `json:"token"`
	Repository  string `json:"repository"`
	PullRequest int    `json:"pullRequest"`
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

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		log.Printf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ensure content type is application/json
	if r.Header.Get("Content-Type") != "application/json" {
		log.Printf("Invalid content type: %s", r.Header.Get("Content-Type"))
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse the request body
	var req AnalysisRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Type != "github" && req.Type != "gitlab" {
		sendErrorResponse(w, "Invalid type. Must be 'github' or 'gitlab'", http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		sendErrorResponse(w, "Token is required", http.StatusBadRequest)
		return
	}
	if req.Repository == "" {
		sendErrorResponse(w, "Repository is required", http.StatusBadRequest)
		return
	}
	if req.PullRequest <= 0 {
		sendErrorResponse(w, "Pull request number must be positive", http.StatusBadRequest)
		return
	}

	// Create repository client based on provider
	var repoClient repo.RepositoryClient
	switch req.Type {
	case "github":
		authProvider := auth.NewGitHubAuth(req.Token)
		if err := authProvider.Authenticate(); err != nil {
			sendErrorResponse(w, fmt.Sprintf("GitHub authentication failed: %v", err), http.StatusUnauthorized)
			return
		}
		repoClient = repo.NewGitHubClient(authProvider.GetClient().(*github.Client))
	case "gitlab":
		authProvider := auth.NewGitLabAuth(req.Token)
		if err := authProvider.Authenticate(); err != nil {
			sendErrorResponse(w, fmt.Sprintf("GitLab authentication failed: %v", err), http.StatusUnauthorized)
			return
		}
		repoClient = repo.NewGitLabClient(authProvider.GetClient().(*gitlab.Client))
	}

	// Get pull request information
	prs, err := repoClient.ListPullRequests(req.Repository)
	if err != nil {
		sendErrorResponse(w, fmt.Sprintf("Failed to get pull requests: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the specific pull request
	var selectedPR *repo.PullRequest
	for _, pr := range prs {
		if pr.Number == req.PullRequest {
			selectedPR = &pr
			break
		}
	}

	if selectedPR == nil {
		sendErrorResponse(w, fmt.Sprintf("Pull request #%d not found", req.PullRequest), http.StatusNotFound)
		return
	}

	// Get blame information
	blameInfo, err := repoClient.GetBlameInfo(req.Repository, req.PullRequest, selectedPR.ChangedFiles)
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
