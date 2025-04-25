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
