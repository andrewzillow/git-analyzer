package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_Validate(t *testing.T) {
	server := NewServer(8080)

	tests := []struct {
		name           string
		method         string
		contentType    string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:        "valid git-blame request",
			method:      http.MethodPost,
			contentType: "application/json",
			requestBody: AnalysisRequest{
				Name: "git-blame",
				Arguments: map[string]interface{}{
					"provider":    "github",
					"token":       "token",
					"repository":  "owner/repo",
					"pullRequest": 1,
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:        "valid git-log request",
			method:      http.MethodPost,
			contentType: "application/json",
			requestBody: AnalysisRequest{
				Name: "git-log",
				Arguments: map[string]interface{}{
					"provider":    "github",
					"token":       "token",
					"repository":  "owner/repo",
					"pullRequest": 1,
				},
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:        "invalid name",
			method:      http.MethodPost,
			contentType: "application/json",
			requestBody: AnalysisRequest{
				Name: "invalid-name",
				Arguments: map[string]interface{}{
					"provider":    "github",
					"token":       "token",
					"repository":  "owner/repo",
					"pullRequest": 1,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid name",
		},
		{
			name:           "invalid method",
			method:         http.MethodGet,
			contentType:    "application/json",
			requestBody:    AnalysisRequest{},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method not allowed",
		},
		{
			name:           "invalid content type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			requestBody:    AnalysisRequest{},
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "invalid content type",
		},
		{
			name:        "invalid provider type",
			method:      http.MethodPost,
			contentType: "application/json",
			requestBody: AnalysisRequest{
				Name: "git-blame",
				Arguments: map[string]interface{}{
					"provider":    "invalid",
					"token":       "token",
					"repository":  "owner/repo",
					"pullRequest": 1,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid provider type",
		},
		{
			name:        "missing token",
			method:      http.MethodPost,
			contentType: "application/json",
			requestBody: AnalysisRequest{
				Name: "git-blame",
				Arguments: map[string]interface{}{
					"provider":    "github",
					"repository":  "owner/repo",
					"pullRequest": 1,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "token is required",
		},
		{
			name:        "missing repository",
			method:      http.MethodPost,
			contentType: "application/json",
			requestBody: AnalysisRequest{
				Name: "git-blame",
				Arguments: map[string]interface{}{
					"provider":    "github",
					"token":       "token",
					"pullRequest": 1,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "repository is required",
		},
		{
			name:        "invalid pull request number",
			method:      http.MethodPost,
			contentType: "application/json",
			requestBody: AnalysisRequest{
				Name: "git-blame",
				Arguments: map[string]interface{}{
					"provider":    "github",
					"token":       "token",
					"repository":  "owner/repo",
					"pullRequest": 0,
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "pull request number must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body
			body, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			// Create request
			req := httptest.NewRequest(tt.method, "/messages", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", tt.contentType)

			// Create response recorder
			w := httptest.NewRecorder()

			// Call validate
			_, err = server.validate(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check error message if expected
			if tt.expectedError != "" {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error, got '%s'", err.Error())
			}
		})
	}
}

func TestServer_HandlePrompts(t *testing.T) {
	server := NewServer(8080)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "valid GET request",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid method",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/prompts", nil)
			w := httptest.NewRecorder()

			server.handlePrompts(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var prompts []Prompt
				if err := json.NewDecoder(w.Body).Decode(&prompts); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				// Verify the structure of the response
				if len(prompts) != 2 {
					t.Errorf("Expected 2 prompts, got %d", len(prompts))
				}

				// Check git-blame prompt
				blamePrompt := prompts[0]
				if blamePrompt.Name != "git-blame" {
					t.Errorf("Expected first prompt to be git-blame, got %s", blamePrompt.Name)
				}
				if len(blamePrompt.Arguments) != 4 {
					t.Errorf("Expected 4 arguments for git-blame, got %d", len(blamePrompt.Arguments))
				}

				// Check git-log prompt
				logPrompt := prompts[1]
				if logPrompt.Name != "git-log" {
					t.Errorf("Expected second prompt to be git-log, got %s", logPrompt.Name)
				}
				if len(logPrompt.Arguments) != 4 {
					t.Errorf("Expected 4 arguments for git-log, got %d", len(logPrompt.Arguments))
				}
			}
		})
	}
}
