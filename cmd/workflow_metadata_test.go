package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseJSONError(t *testing.T) {
	tests := []struct {
		name        string
		inputError  error
		jsonContent string
		contextName string
		expected    string
	}{
		// Test invalid character in string literal
		{
			name:        "invalid character in string literal",
			inputError:  errors.New(`invalid character '"' in string literal`),
			jsonContent: `{"name": "test"workflow", "version": 1}`,
			contextName: "workflow definition",
			expected:    "JSON syntax error in workflow definition",
		},
		// Test unterminated string detection
		{
			name:        "unterminated string",
			inputError:  errors.New(`invalid character 'w' in string literal`),
			jsonContent: "{\n\"name\": \"test workflow,\n\"version\": 1\n}",
			contextName: "workflow definition",
			expected:    "unterminated string on line 2",
		},
		// Test unexpected end of JSON input
		{
			name:        "unexpected end of JSON",
			inputError:  errors.New("unexpected end of JSON input"),
			jsonContent: `{"name": "test"`,
			contextName: "task definition",
			expected:    "unexpected end of file",
		},
		// Test invalid character general case
		{
			name:        "invalid character general",
			inputError:  errors.New("invalid character '}' looking for beginning of value"),
			jsonContent: `{"name": "test", }`,
			contextName: "workflow definition",
			expected:    "invalid characters, missing commas, or malformed values",
		},
		// Test fallback for other JSON errors
		{
			name:        "other JSON error",
			inputError:  errors.New("cannot unmarshal string into Go struct field"),
			jsonContent: `{"name": 123}`,
			contextName: "task definition",
			expected:    "Invalid task definition format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJSONError(tt.inputError, tt.jsonContent, tt.contextName)
			
			if !strings.Contains(result.Error(), tt.expected) {
				t.Errorf("expected error to contain '%s', got '%s'", tt.expected, result.Error())
			}
		})
	}
}

func TestParseJSONError_QuoteCountLogic(t *testing.T) {
	// Test the quote counting logic specifically
	tests := []struct {
		name        string
		jsonContent string
		expectLine  int
	}{
		{
			name: "missing closing quote on line 2",
			jsonContent: `{
"name": "test workflow,
"version": 1
}`,
			expectLine: 2,
		},
		{
			name: "missing closing quote on line 3",
			jsonContent: `{
"name": "test",
"description": "workflow without closing quote,
"version": 1
}`,
			expectLine: 3,
		},
		{
			name: "escaped quotes should not trigger",
			jsonContent: `{
"name": "test \"quoted\" value",
"version": 1
}`,
			expectLine: -1, // Should not find any problematic line
		},
	}

	baseError := errors.New(`invalid character 'w' in string literal`)
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJSONError(baseError, tt.jsonContent, "workflow definition")
			
			if tt.expectLine > 0 {
				if !strings.Contains(result.Error(), "line") {
					t.Errorf("expected error to mention line number, got '%s'", result.Error())
				}
			} else {
				// Should fall back to general syntax error
				if !strings.Contains(result.Error(), "unterminated strings") {
					t.Errorf("expected fallback error message, got '%s'", result.Error())
				}
			}
		})
	}
}

func TestParseAPIError(t *testing.T) {
	tests := []struct {
		name        string
		inputError  error
		defaultMsg  string
		expected    []string // Multiple strings that should be in the result
	}{
		// Test error with body JSON
		{
			name: "error with body JSON",
			inputError: errors.New(`error: {"status":400,"message":"Invalid request"}, body: {"status":400,"message":"Workflow validation failed","validationErrors":[{"path":"tasks[0].name","message":"Task name is required"}]}`),
			defaultMsg: "Failed to create workflow",
			expected:   []string{"Failed to create workflow", "Workflow validation failed", "tasks[0].name: Task name is required", "status: 400"},
		},
		// Test error with simple error JSON
		{
			name:       "error with simple error JSON",
			inputError: errors.New(`error: {"message":"Unauthorized access"}, body: {}`),
			defaultMsg: "Failed to update workflow",
			expected:   []string{"Failed to update workflow", "Unauthorized access"},
		},
		// Test error with validation errors but no path
		{
			name:       "validation errors without path",
			inputError: errors.New(`body: {"message":"Validation failed","validationErrors":[{"message":"Invalid workflow structure"}]}`),
			defaultMsg: "Failed to validate",
			expected:   []string{"Failed to validate", "Validation failed", "Invalid workflow structure"},
		},
		// Test error with malformed JSON
		{
			name:       "malformed JSON in error",
			inputError: errors.New(`error: {invalid json}, body: {}`),
			defaultMsg: "Failed to process",
			expected:   []string{"Failed to process"},
		},
		// Test error without JSON structure
		{
			name:       "plain error message",
			inputError: errors.New("connection timeout"),
			defaultMsg: "Failed to connect",
			expected:   []string{"Failed to connect", "connection timeout"},
		},
		// Test empty error message in JSON
		{
			name:       "empty message in JSON",
			inputError: errors.New(`body: {"status":500}`),
			defaultMsg: "Server error",
			expected:   []string{"Server error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAPIError(tt.inputError, tt.defaultMsg)
			
			for _, expectedText := range tt.expected {
				if !strings.Contains(result.Error(), expectedText) {
					t.Errorf("expected error to contain '%s', got '%s'", expectedText, result.Error())
				}
			}
		})
	}
}

func TestParseAPIError_JSONStructures(t *testing.T) {
	// Test specific JSON structures that the function should handle
	tests := []struct {
		name         string
		errorJSON    map[string]interface{}
		errorFormat  string // "error" or "body"
		defaultMsg   string
		expectStatus bool
		expectVE     bool // expect validation errors
	}{
		{
			name: "complete error response",
			errorJSON: map[string]interface{}{
				"status":  400,
				"message": "Validation failed",
				"validationErrors": []map[string]interface{}{
					{"path": "name", "message": "Name is required"},
					{"path": "version", "message": "Version must be positive"},
				},
			},
			errorFormat:  "body",
			defaultMsg:   "Failed operation",
			expectStatus: true,
			expectVE:     true,
		},
		{
			name: "error response without validation errors",
			errorJSON: map[string]interface{}{
				"status":  404,
				"message": "Workflow not found",
			},
			errorFormat:  "error",
			defaultMsg:   "Failed to find",
			expectStatus: false, // The current implementation doesn't show status for "error:" format
			expectVE:     false,
		},
		{
			name: "minimal error response",
			errorJSON: map[string]interface{}{
				"message": "Something went wrong",
			},
			errorFormat:  "body",
			defaultMsg:   "Operation failed",
			expectStatus: false,
			expectVE:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the JSON
			jsonBytes, err := json.Marshal(tt.errorJSON)
			if err != nil {
				t.Fatalf("failed to marshal test JSON: %v", err)
			}

			// Create error string in expected format
			var errorStr string
			if tt.errorFormat == "body" {
				errorStr = "some error, body: " + string(jsonBytes)
			} else {
				errorStr = "error: " + string(jsonBytes) + ", body: {}"
			}

			inputError := errors.New(errorStr)
			result := parseAPIError(inputError, tt.defaultMsg)
			errorText := result.Error()

			// Check that default message is included
			if !strings.Contains(errorText, tt.defaultMsg) {
				t.Errorf("expected error to contain default message '%s', got '%s'", tt.defaultMsg, errorText)
			}

			// Check that main message is included
			if msg, ok := tt.errorJSON["message"].(string); ok && msg != "" {
				if !strings.Contains(errorText, msg) {
					t.Errorf("expected error to contain message '%s', got '%s'", msg, errorText)
				}
			}

			// Check status if expected
			if tt.expectStatus {
				if !strings.Contains(errorText, "status:") {
					t.Errorf("expected error to contain status, got '%s'", errorText)
				}
			}

			// Check validation errors if expected
			if tt.expectVE {
				if !strings.Contains(errorText, "Validation errors:") {
					t.Errorf("expected error to contain validation errors section, got '%s'", errorText)
				}
			}
		})
	}
}