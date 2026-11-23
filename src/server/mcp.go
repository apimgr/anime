package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/apimgr/anime/src/common/i18n"
)

const mcpProtocolVersion = "2026-07-28"

type mcpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Annotations map[string]bool        `json:"annotations,omitempty"`
	path        string
	method      string
	pathParams  map[string]bool
	queryParams map[string]bool
	bodyParams  map[string]bool
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpSourceContextKey struct{}

func withMCPSource(ctx context.Context) context.Context {
	return context.WithValue(ctx, mcpSourceContextKey{}, true)
}

func isMCPSource(r *http.Request) bool {
	if r == nil {
		return false
	}
	value, _ := r.Context().Value(mcpSourceContextKey{}).(bool)
	return value
}

func mcpToolRegistry() []mcpTool {
	document := openAPIDocument()
	paths, ok := document["paths"].(map[string]interface{})
	if !ok {
		return nil
	}
	pathNames := make([]string, 0, len(paths))
	for path := range paths {
		pathNames = append(pathNames, path)
	}
	sort.Strings(pathNames)
	tools := make([]mcpTool, 0)
	for _, path := range pathNames {
		rawPathItem := paths[path]
		pathItem, ok := rawPathItem.(map[string]interface{})
		if !ok {
			continue
		}
		methods := make([]string, 0, len(pathItem))
		for method := range pathItem {
			methods = append(methods, method)
		}
		sort.Strings(methods)
		for _, method := range methods {
			rawOperation := pathItem[method]
			operation, ok := rawOperation.(map[string]interface{})
			if !ok || operation["x-mcp-enabled"] != true {
				continue
			}
			name, _ := operation["operationId"].(string)
			description, _ := operation["summary"].(string)
			if name == "" || description == "" {
				continue
			}
			properties := make(map[string]interface{})
			required := make([]string, 0)
			pathParams := make(map[string]bool)
			queryParams := make(map[string]bool)
			bodyParams := make(map[string]bool)
			if parameters, ok := operation["parameters"].([]interface{}); ok {
				for _, rawParameter := range parameters {
					parameter, ok := rawParameter.(map[string]interface{})
					if !ok || parameter["in"] != "query" && parameter["in"] != "path" {
						continue
					}
					parameterName, _ := parameter["name"].(string)
					if parameterName != "" {
						properties[parameterName] = parameter["schema"]
						if parameter["in"] == "path" {
							pathParams[parameterName] = true
						} else {
							queryParams[parameterName] = true
						}
						if parameter["required"] == true {
							required = append(required, parameterName)
						}
					}
				}
			}
			if requestBody, ok := operation["requestBody"].(map[string]interface{}); ok {
				if content, ok := requestBody["content"].(map[string]interface{}); ok {
					if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
						if schema, ok := jsonContent["schema"].(map[string]interface{}); ok {
							if bodyProperties, ok := schema["properties"].(map[string]interface{}); ok {
								for property, definition := range bodyProperties {
									properties[property] = definition
									bodyParams[property] = true
								}
							}
							if bodyRequired, ok := schema["required"].([]string); ok {
								required = append(required, bodyRequired...)
							}
						}
					}
				}
			}
			httpMethod := strings.ToUpper(method)
			annotations := make(map[string]bool)
			switch httpMethod {
			case http.MethodGet:
				annotations["readOnlyHint"] = true
			case http.MethodDelete:
				annotations["destructiveHint"] = true
				annotations["idempotentHint"] = true
			case http.MethodPut:
				annotations["idempotentHint"] = true
			case http.MethodPost, http.MethodPatch:
				annotations["readOnlyHint"] = false
			}
			tools = append(tools, mcpTool{
				Name: name, Description: description,
				InputSchema: map[string]interface{}{"type": "object", "properties": properties, "required": required, "additionalProperties": false},
				Annotations: annotations, path: path, method: httpMethod,
				pathParams: pathParams, queryParams: queryParams, bodyParams: bodyParams,
			})
		}
	}
	return tools
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if !s.validMCPOrigin(r) {
		writeProblem(w, http.StatusForbidden, "origin is not allowed")
		return
	}
	if _, ok := s.mcpOwnerIdentity(r); !ok {
		s.writeMCPUnauthorized(w)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeProblem(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	mediaType, _, mediaTypeErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaTypeErr != nil || !strings.EqualFold(mediaType, "application/json") {
		writeProblem(w, http.StatusBadRequest, "MCP requests must use application/json")
		return
	}
	var request mcpRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, http.StatusBadRequest, "a valid JSON-RPC 2.0 request is required")
		return
	}
	if request.JSONRPC != "2.0" || request.Method == "" {
		s.writeMCPError(w, request.ID, -32600, "Invalid Request", http.StatusBadRequest)
		return
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		s.writeMCPError(w, request.ID, -32600, "Invalid Request: a single JSON-RPC request is required", http.StatusBadRequest)
		return
	}
	version := r.Header.Get("MCP-Protocol-Version")
	if version != "" && version != mcpProtocolVersion || request.Method == "initialize" && version == "" {
		s.writeMCPError(w, request.ID, -32600, "UnsupportedProtocolVersionError: supported version is "+mcpProtocolVersion, http.StatusBadRequest)
		return
	}
	if version == "" {
		s.writeMCPError(w, request.ID, -32020, "HeaderMismatch", http.StatusBadRequest)
		return
	}
	if !s.validMCPHeaders(r, request) {
		s.writeMCPError(w, request.ID, -32020, "HeaderMismatch", http.StatusBadRequest)
		return
	}
	var result interface{}
	switch request.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil || params.ProtocolVersion == "" {
			s.writeMCPError(w, request.ID, -32602, "protocolVersion is required", http.StatusBadRequest)
			return
		}
		if params.ProtocolVersion != mcpProtocolVersion {
			s.writeMCPError(w, request.ID, -32600, "UnsupportedProtocolVersionError: supported version is "+mcpProtocolVersion, http.StatusBadRequest)
			return
		}
		result = map[string]interface{}{"protocolVersion": mcpProtocolVersion, "capabilities": map[string]interface{}{"tools": map[string]bool{"listChanged": false}}, "serverInfo": map[string]string{"name": "anime-quotes-api", "version": Version}}
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
		return
	case "ping":
		result = map[string]interface{}{}
	case "tools/list":
		result = map[string]interface{}{"tools": mcpToolRegistry()}
	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil {
			s.writeMCPError(w, request.ID, -32602, "invalid tool arguments", http.StatusOK)
			return
		}
		tool, ok := findMCPTool(params.Name)
		if !ok {
			s.writeMCPError(w, request.ID, -32602, "unknown tool", http.StatusOK)
			return
		}
		result = s.callMCPTool(r, tool, params.Arguments)
	default:
		s.writeMCPError(w, request.ID, -32601, "method not found", http.StatusNotFound)
		return
	}
	s.writeMCPResult(w, request.ID, result)
}

func findMCPTool(name string) (mcpTool, bool) {
	for _, tool := range mcpToolRegistry() {
		if tool.Name == name {
			return tool, true
		}
	}
	return mcpTool{}, false
}

func (s *Server) callMCPTool(parent *http.Request, tool mcpTool, arguments map[string]interface{}) interface{} {
	query := url.Values{}
	path := tool.path
	for key, value := range arguments {
		if text, ok := value.(string); ok {
			if tool.pathParams[key] {
				path = strings.ReplaceAll(path, "{"+key+"}", url.PathEscape(text))
			} else if tool.queryParams[key] {
				query.Set(key, text)
			}
		}
	}
	method := tool.method
	if method == "" {
		method = http.MethodGet
	}
	target := path
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	request := parent.Clone(parent.Context())
	request = request.WithContext(withMCPSource(request.Context()))
	request.Method = method
	request.URL.Path = path
	request.URL.RawPath = ""
	request.URL.RawQuery = query.Encode()
	request.RequestURI = target
	request.Header.Del("Cookie")
	if len(tool.bodyParams) > 0 {
		body := make(map[string]interface{}, len(tool.bodyParams))
		for key := range tool.bodyParams {
			if value, ok := arguments[key]; ok {
				body[key] = value
			}
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return map[string]interface{}{"content": []interface{}{map[string]interface{}{"type": "text", "text": "invalid tool arguments"}}, "isError": true}
		}
		request.Body = io.NopCloser(bytes.NewReader(payload))
		request.ContentLength = int64(len(payload))
		request.Header.Set("Content-Type", "application/json")
	}
	response := &mcpRecorder{header: make(http.Header)}
	s.router.ServeHTTP(response, request)
	if response.status == 0 {
		response.status = http.StatusOK
	}
	var body interface{}
	if err := json.Unmarshal(response.body.Bytes(), &body); err != nil {
		body = strings.TrimSpace(response.body.String())
	}
	serialized, err := json.Marshal(body)
	if err != nil {
		serialized = []byte(fmt.Sprint(body))
	}
	return map[string]interface{}{"content": []interface{}{map[string]interface{}{"type": "text", "text": string(serialized)}}, "isError": response.status >= 400}
}

type mcpRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *mcpRecorder) Header() http.Header    { return r.header }
func (r *mcpRecorder) WriteHeader(status int) { r.status = status }
func (r *mcpRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(data)
}

func (s *Server) validMCPOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.Scheme == "" || parsed.User != nil || parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if strings.EqualFold(origin, requestOrigin(r)) {
		return true
	}
	if s.cfg.Server.FQDN != "" && strings.EqualFold(parsed.Host, s.cfg.Server.FQDN) {
		return parsed.Scheme == "https" || parsed.Scheme == "http"
	}
	if OfficialSite != "" {
		publicOrigin, parseErr := url.Parse(OfficialSite)
		if parseErr == nil && strings.EqualFold(publicOrigin.Scheme, parsed.Scheme) && strings.EqualFold(publicOrigin.Host, parsed.Host) {
			return true
		}
	}
	for _, configured := range strings.Split(s.cfg.WebSecurity.CORS, ",") {
		allowed, parseErr := url.Parse(strings.TrimSpace(configured))
		if parseErr == nil && allowed.Scheme != "" && allowed.Host != "" && strings.EqualFold(allowed.Scheme, parsed.Scheme) && strings.EqualFold(allowed.Host, parsed.Host) {
			return true
		}
	}
	return false
}

func (s *Server) validMCPHeaders(r *http.Request, request mcpRequest) bool {
	method := r.Header.Get("Mcp-Method")
	if method == "" || method != request.Method {
		return false
	}
	var params map[string]interface{}
	_ = json.Unmarshal(request.Params, &params)
	if request.Method == "tools/call" || request.Method == "resources/read" || request.Method == "prompts/get" {
		name := params["name"]
		if request.Method == "resources/read" {
			name = params["uri"]
		}
		want, _ := name.(string)
		got := r.Header.Get("Mcp-Name")
		if strings.HasPrefix(got, "=?base64?") && strings.HasSuffix(got, "?=") {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(got, "=?base64?"), "?="))
			if err != nil {
				return false
			}
			got = string(decoded)
		}
		if want == "" || got != want {
			return false
		}
	} else if r.Header.Get("Mcp-Name") != "" {
		return false
	}
	return true
}

func (s *Server) writeMCPResult(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(mcpResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) writeMCPError(w http.ResponseWriter, id json.RawMessage, code int, message string, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpError{Code: code, Message: message}})
}

type mcpHelpSection struct {
	ID      string
	Title   string
	Content string
	Code    string
}

func (s *Server) mcpHelpData(r *http.Request) map[string]interface{} {
	language := s.resolvedLanguage(r)
	translate := func(key string) string { return i18n.Translate(language, "mcp_page."+key) }
	translateFormat := func(key string, values map[string]string) string {
		return i18n.TranslateFormat(language, "mcp_page."+key, values)
	}
	url := requestOrigin(r) + "/api/v1/server/mcp"
	origin := requestOrigin(r)
	sections := []mcpHelpSection{
		{ID: "overview", Title: translate("section_overview"), Content: translate("overview")},
		{ID: "endpoint", Title: translate("section_endpoint"), Content: translateFormat("endpoint_detail", map[string]string{"url": url, "api_alias": origin + "/api/mcp", "short_alias": origin + "/mcp"})},
		{ID: "authentication", Title: translate("section_authentication"), Content: translate("authentication")},
		{ID: "connect", Title: translate("section_connect"), Code: "claude mcp add --transport http anime " + url + " --header \"Authorization: Bearer $MCP_TOKEN\"\n\n{\n  \"mcpServers\": {\n    \"anime\": {\n      \"type\": \"http\",\n      \"url\": \"" + url + "\",\n      \"headers\": {\"Authorization\": \"Bearer $MCP_TOKEN\"}\n    }\n  }\n}\n\ncurl -sS -X POST '" + url + "' -H 'Content-Type: application/json' -H 'Authorization: Bearer $MCP_TOKEN' -H 'MCP-Protocol-Version: " + mcpProtocolVersion + "' -H 'Mcp-Method: initialize' --data '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"" + mcpProtocolVersion + "\",\"capabilities\":{},\"clientInfo\":{\"name\":\"example\",\"version\":\"1.0\"}}}'"},
		{ID: "tools", Title: translate("section_tools"), Content: translate("tools_content")},
		{ID: "limits", Title: translate("section_limits"), Content: translateFormat("limits", map[string]string{"version": mcpProtocolVersion})},
		{ID: "troubleshooting", Title: translate("section_troubleshooting"), Content: translate("troubleshooting")},
	}
	tools := mcpToolRegistry()
	project := translate("project_content")
	statuses := []map[string]string{
		{"Code": "400", "Meaning": translate("status_400_meaning"), "Fix": translateFormat("status_400_fix", map[string]string{"version": mcpProtocolVersion})},
		{"Code": "401", "Meaning": translate("status_401_meaning"), "Fix": translate("status_401_fix")},
		{"Code": "403", "Meaning": translate("status_403_meaning"), "Fix": translate("status_403_fix")},
		{"Code": "405", "Meaning": translate("status_405_meaning"), "Fix": translate("status_405_fix")},
	}
	return map[string]interface{}{"Title": translate("title"), "Description": translate("lead"), "Page": "help-mcp", "ContentTemplate": "mcp-content", "Version": Version, "MCPURL": url, "MCPSections": sections, "MCPTools": tools, "MCPStatuses": statuses, "MCPProject": project, "MCPProjectTitle": translate("project_title"), "MCPLanguage": language}
}

func (s *Server) handleMCPHelpPage(w http.ResponseWriter, r *http.Request) {
	if err := s.renderPage(w, r, s.mcpHelpData(r)); err != nil {
		writeFallbackError(w, r, http.StatusInternalServerError)
	}
}

func (s *Server) handleMCPHelpJSON(w http.ResponseWriter, r *http.Request) {
	origin := requestOrigin(r)
	language := s.resolvedLanguage(r)
	tools := mcpToolRegistry()
	entries := make([]map[string]string, 0, len(tools))
	for _, tool := range tools {
		entries = append(entries, map[string]string{"name": tool.Name, "description": tool.Description})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"enabled": true, "url": origin + "/api/v1/server/mcp", "aliases": []string{origin + "/api/mcp", origin + "/mcp"}, "auth": map[string]string{"scheme": "Bearer", "required": i18n.Translate(language, "mcp_page.every_request"), "credential": i18n.Translate(language, "mcp_page.same_api_token")}, "tools": entries, "project": i18n.Translate(language, "mcp_page.project_content")})
}
