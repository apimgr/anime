package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPInitializeAndTools(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	request.Header.Set("Mcp-Method", "initialize")
	response := httptest.NewRecorder()
	request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"protocolVersion":"2026-07-28"`) {
		t.Fatalf("initialize response status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	request.Header.Set("Mcp-Method", "tools/list")
	response = httptest.NewRecorder()
	request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"getRandomQuote"`) {
		t.Fatalf("tools/list response status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMCPToolCallUsesPublicQuoteHandler(t *testing.T) {
	s := newTestServer(t)
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"listQuotes","arguments":{"anime":"naruto"}}}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/mcp", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	request.Header.Set("Mcp-Method", "tools/call")
	request.Header.Set("Mcp-Name", "listQuotes")
	response := httptest.NewRecorder()
	request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("tools/call status=%d body=%s", response.Code, response.Body.String())
	}
	var result mcpResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode tools/call response: %v", err)
	}
	if result.Error != nil || !strings.Contains(response.Body.String(), "Believe it!") {
		t.Fatalf("unexpected tools/call response: %s", response.Body.String())
	}
}

func TestMCPRejectsUnsupportedProtocolAndOrigin(t *testing.T) {
	s := newTestServer(t)
	for _, test := range []struct {
		name   string
		header string
		value  string
		status int
	}{
		{name: "protocol", header: "MCP-Protocol-Version", value: "2025-03-26", status: http.StatusBadRequest},
		{name: "origin", header: "Origin", value: "https://attacker.invalid", status: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/server/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Mcp-Method", "tools/list")
			if test.name == "origin" {
				request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
			}
			request.Header.Set(test.header, test.value)
			response := httptest.NewRecorder()
			request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
			s.router.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Header().Get("Content-Type"), "json") {
				t.Fatalf("status=%d content-type=%q body=%s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
			}
		})
	}
}

func TestMCPHelpJSONListsLiveTools(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/server/help/mcp", nil)
	response := httptest.NewRecorder()
	request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":true`) || !strings.Contains(response.Body.String(), `"name":"listQuotes"`) {
		t.Fatalf("help response status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMCPRouteNeverNegotiatesToPlainText(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":5,"method":"initialize","params":{}}`))
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	request.Header.Set("Mcp-Method", "initialize")
	response := httptest.NewRecorder()
	request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
	s.router.ServeHTTP(response, request)
	if !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("MCP response content type=%q body=%s", response.Header().Get("Content-Type"), response.Body.String())
	}
}

func TestMCPToolsAreDerivedFromOpenAPI(t *testing.T) {
	document := openAPIDocument()
	paths := document["paths"].(map[string]interface{})
	operations := map[string]map[string]interface{}{}
	for path, rawItem := range paths {
		item := rawItem.(map[string]interface{})
		for method, rawOperation := range item {
			operation := rawOperation.(map[string]interface{})
			if operation["x-mcp-enabled"] == true {
				operations[operation["operationId"].(string)] = map[string]interface{}{"path": path, "method": method, "operation": operation}
			}
		}
	}
	tools := mcpToolRegistry()
	if len(tools) != len(operations) {
		t.Fatalf("MCP tools=%d, enabled OpenAPI operations=%d", len(tools), len(operations))
	}
	for _, tool := range tools {
		entry, ok := operations[tool.Name]
		if !ok {
			t.Errorf("MCP tool %q has no enabled OpenAPI operation", tool.Name)
			continue
		}
		operation := entry["operation"].(map[string]interface{})
		if tool.path != entry["path"] || tool.Description != operation["summary"] {
			t.Errorf("MCP tool %q does not match OpenAPI operation: %#v", tool.Name, entry)
		}
	}
}

func TestMCPRequiresMatchingProtocolAndMethodHeaders(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":6,"method":"tools/list","params":{}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	response := httptest.NewRecorder()
	request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":-32020`) {
		t.Fatalf("missing MCP method header status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMCPUnknownMethodAndHTTPMethodsAreJSONErrors(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/server/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"unknown/method","params":{}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	request.Header.Set("Mcp-Method", "unknown/method")
	response := httptest.NewRecorder()
	request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Header().Get("Content-Type"), "json") || !strings.Contains(response.Body.String(), `"code":-32601`) {
		t.Fatalf("unknown method status=%d content-type=%q body=%s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/mcp", nil)
	response = httptest.NewRecorder()
	request.Header.Set("Authorization", "Bearer adm_0123456789abcdefghijklmnopqrstuv")
	s.router.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodPost || !strings.Contains(response.Header().Get("Content-Type"), "json") {
		t.Fatalf("GET /mcp status=%d allow=%q content-type=%q body=%s", response.Code, response.Header().Get("Allow"), response.Header().Get("Content-Type"), response.Body.String())
	}
}

func TestOpenAPIMCPContractDefinesAliasesAndBearerHeaders(t *testing.T) {
	document := openAPIDocument()
	paths := document["paths"].(map[string]interface{})
	for _, path := range []string{"/api/v1/server/mcp", "/api/mcp", "/mcp"} {
		item := paths[path].(map[string]interface{})
		operation := item["post"].(map[string]interface{})
		if len(operation["security"].([]interface{})) == 0 {
			t.Errorf("%s does not declare bearer auth", path)
		}
		params := operation["parameters"].([]interface{})
		seen := map[string]bool{}
		for _, raw := range params {
			seen[raw.(map[string]interface{})["name"].(string)] = true
		}
		for _, header := range []string{"MCP-Protocol-Version", "Mcp-Method", "Mcp-Name"} {
			if !seen[header] {
				t.Errorf("%s OpenAPI operation is missing header %s", path, header)
			}
		}
	}
}
