package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOwnerTokenIssueAndLinkedTokenRoutes(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences/tokens/primary", nil)
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("issue status=%d body=%s", response.Code, response.Body.String())
	}
	var issued struct {
		Data struct {
			OwnerToken string `json:"owner_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &issued); err != nil || !validOwnerTokenFormat(issued.Data.OwnerToken) {
		t.Fatalf("issued token response invalid: %v", err)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/server/preferences/tokens", nil)
	request.Header.Set("Authorization", "Bearer "+issued.Data.OwnerToken)
	response = httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"tokens"`) {
		t.Fatalf("token list status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/server/preferences/tokens", strings.NewReader(`{"name":"test client","expiry":"30d"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+issued.Data.OwnerToken)
	response = httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"name":"test client"`) {
		t.Fatalf("linked token creation status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOwnerTokenRoutesAreNotMCPTools(t *testing.T) {
	for _, tool := range mcpToolRegistry() {
		if strings.Contains(strings.ToLower(tool.Name), "token") {
			t.Errorf("token management exposed as MCP tool %q", tool.Name)
		}
	}
	paths := openAPIDocument()["paths"].(map[string]interface{})
	for path, raw := range paths {
		if !strings.Contains(path, "/preferences/tokens") {
			continue
		}
		for _, operation := range raw.(map[string]interface{}) {
			if operation.(map[string]interface{})["x-mcp-enabled"] == true {
				t.Errorf("token management operation %s exposed as MCP tool", path)
			}
		}
	}
}

func TestMCPRequiresBearerAuthenticationBeforeProtocolChecks(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") == "" || !strings.Contains(response.Body.String(), `"error"`) {
		t.Fatalf("unauthenticated MCP response status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}

func TestOwnerTokenCookieMaxAgeUsesConfiguredDuration(t *testing.T) {
	for value, want := range map[string]time.Duration{"1y": 365 * 24 * time.Hour, "30d": 30 * 24 * time.Hour, "2h": 2 * time.Hour, "invalid": 365 * 24 * time.Hour} {
		if got := ownerTokenCookieMaxAge(value); got != want {
			t.Errorf("ownerTokenCookieMaxAge(%q)=%s, want %s", value, got, want)
		}
	}
}
