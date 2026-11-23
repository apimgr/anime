package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/graphql-go/graphql"
)

type graphQLRequest struct {
	Query string `json:"query"`
}

// openAPIDocument is the single operation registry used by OpenAPI and MCP.
func openAPIDocument() map[string]interface{} {
	quoteResponse := map[string]interface{}{"200": map[string]interface{}{"description": "JSON response envelope", "content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{"$ref": "#/components/schemas/QuoteResponse"}}}}}
	listResponse := map[string]interface{}{"200": map[string]interface{}{"description": "JSON response envelope containing matching quotes"}}
	textResponse := map[string]interface{}{"200": map[string]interface{}{"description": "Plain text response", "content": map[string]interface{}{"text/plain": map[string]interface{}{"schema": map[string]interface{}{"type": "string"}}}}}
	stringParam := func(name string) map[string]interface{} {
		return map[string]interface{}{"name": name, "in": "query", "schema": map[string]interface{}{"type": "string", "maxLength": 256}}
	}
	graphqlGet := map[string]interface{}{"summary": "Execute a read-only GraphQL query", "parameters": []interface{}{map[string]interface{}{"name": "query", "in": "query", "required": true, "schema": map[string]interface{}{"type": "string"}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "GraphQL result with data and optional errors"}}}
	graphqlPost := map[string]interface{}{"summary": "Execute a read-only GraphQL query", "requestBody": map[string]interface{}{"required": true, "content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "required": []string{"query"}, "properties": map[string]interface{}{"query": map[string]interface{}{"type": "string"}}}}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "GraphQL result with data and optional errors"}}}
	mcpOperation := map[string]interface{}{"operationId": "mcp", "summary": "MCP Streamable HTTP endpoint", "description": "Uses the API Bearer credential and the required MCP protocol and method headers.", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "parameters": []interface{}{map[string]interface{}{"name": "MCP-Protocol-Version", "in": "header", "required": true, "schema": map[string]interface{}{"type": "string", "const": "2026-07-28"}}, map[string]interface{}{"name": "Mcp-Method", "in": "header", "required": true, "schema": map[string]interface{}{"type": "string"}}, map[string]interface{}{"name": "Mcp-Name", "in": "header", "required": false, "description": "Required for tools/call, resources/read, and prompts/get; matches the corresponding name or URI in params.", "schema": map[string]interface{}{"type": "string"}}}, "requestBody": map[string]interface{}{"required": true, "content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "required": []string{"jsonrpc", "method"}, "properties": map[string]interface{}{"jsonrpc": map[string]interface{}{"type": "string", "const": "2.0"}, "id": map[string]interface{}{}, "method": map[string]interface{}{"type": "string"}, "params": map[string]interface{}{"type": "object"}}}}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "MCP JSON-RPC response"}, "400": map[string]interface{}{"description": "Invalid request, header mismatch, or unsupported protocol version"}, "401": map[string]interface{}{"description": "A valid API Bearer token is required"}, "403": map[string]interface{}{"description": "Origin is not allowed"}, "404": map[string]interface{}{"description": "Unknown JSON-RPC method"}, "405": map[string]interface{}{"description": "Only POST is supported"}}}
	paths := map[string]interface{}{
		"/api/v1":                                map[string]interface{}{"get": map[string]interface{}{"summary": "API route index", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Available API routes"}}}},
		"/api":                                   map[string]interface{}{"get": map[string]interface{}{"summary": "Unversioned API route index alias", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Available API routes"}}}},
		"/api/v1/random":                         map[string]interface{}{"get": map[string]interface{}{"operationId": "getRandomQuote", "summary": "Get a random quote", "x-mcp-enabled": true, "responses": quoteResponse}},
		"/api/v1/quotes":                         map[string]interface{}{"get": map[string]interface{}{"operationId": "listQuotes", "summary": "List and search quotes", "x-mcp-enabled": true, "parameters": []interface{}{stringParam("q"), stringParam("anime"), stringParam("character")}, "responses": listResponse}},
		"/api/v1/server/healthz":                 map[string]interface{}{"get": map[string]interface{}{"operationId": "getHealth", "summary": "Health status", "x-mcp-enabled": true, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Healthy service and build information"}, "503": map[string]interface{}{"description": "Service is unhealthy"}}}},
		"/api/v1/server/preferences":             map[string]interface{}{"get": map[string]interface{}{"summary": "Read browser preferences", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Effective theme and language preferences"}}}, "post": map[string]interface{}{"summary": "Save browser preferences", "requestBody": map[string]interface{}{"required": true, "content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "additionalProperties": true, "properties": map[string]interface{}{"theme": map[string]interface{}{"type": "string", "enum": []string{"dark", "light", "auto"}}, "lang": map[string]interface{}{"type": "string"}}}}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Applied and ignored preference names"}, "400": map[string]interface{}{"description": "No valid preference values supplied"}}}},
		"/api/v1/server/preferences/sync":        map[string]interface{}{"get": map[string]interface{}{"summary": "Export browser preferences", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Effective preferences, share URL, and share code"}}}},
		"/api/v1/server/preferences/sync/import": map[string]interface{}{"get": map[string]interface{}{"summary": "Import shared browser preferences", "parameters": []interface{}{map[string]interface{}{"name": "code", "in": "query", "schema": map[string]interface{}{"type": "string", "maxLength": 2048}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Applied and ignored preference names"}}}},
		"/api/v1/server/preferences/tokens": map[string]interface{}{
			"get":  map[string]interface{}{"operationId": "listOwnerTokens", "summary": "List owner tokens with a primary token", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Masked token metadata"}, "401": map[string]interface{}{"description": "Primary owner token required"}, "403": map[string]interface{}{"description": "Linked tokens cannot manage tokens"}}},
			"post": map[string]interface{}{"operationId": "createLinkedToken", "summary": "Create a linked owner token", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "requestBody": map[string]interface{}{"required": true, "content": map[string]interface{}{"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object", "required": []string{"name"}, "properties": map[string]interface{}{"name": map[string]interface{}{"type": "string", "minLength": 1, "maxLength": 64}, "expiry": map[string]interface{}{"type": "string"}}}}}}, "responses": map[string]interface{}{"201": map[string]interface{}{"description": "New linked token and metadata"}, "403": map[string]interface{}{"description": "Primary owner token required"}}},
		},
		"/api/v1/server/preferences/tokens/primary":        map[string]interface{}{"post": map[string]interface{}{"operationId": "issueOwnerToken", "summary": "Issue a new primary owner token", "responses": map[string]interface{}{"201": map[string]interface{}{"description": "Owner token, returned once for issuance"}, "409": map[string]interface{}{"description": "A valid token is already present"}}}},
		"/api/v1/server/preferences/tokens/primary/reveal": map[string]interface{}{"post": map[string]interface{}{"operationId": "revealPrimaryToken", "summary": "Reveal the primary token", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Decrypted primary token"}}}},
		"/api/v1/server/preferences/tokens/primary/rotate": map[string]interface{}{"post": map[string]interface{}{"operationId": "rotatePrimaryToken", "summary": "Rotate the primary token in place", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Replacement token"}}}},
		"/api/v1/server/preferences/tokens/{id}":           map[string]interface{}{"patch": map[string]interface{}{"operationId": "updateLinkedToken", "summary": "Rename or change linked-token expiry", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "parameters": []interface{}{map[string]interface{}{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "integer"}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Updated linked token metadata"}}}, "delete": map[string]interface{}{"operationId": "revokeLinkedToken", "summary": "Revoke a linked token", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "parameters": []interface{}{map[string]interface{}{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "integer"}}}, "responses": map[string]interface{}{"204": map[string]interface{}{"description": "Token revoked"}}}},
		"/api/v1/server/preferences/tokens/{id}/reveal":    map[string]interface{}{"post": map[string]interface{}{"operationId": "revealLinkedToken", "summary": "Reveal a linked token", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "parameters": []interface{}{map[string]interface{}{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "integer"}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Decrypted linked token"}}}},
		"/api/v1/server/preferences/tokens/{id}/rotate":    map[string]interface{}{"post": map[string]interface{}{"operationId": "rotateLinkedToken", "summary": "Rotate a linked token in place", "security": []interface{}{map[string]interface{}{"BearerAuth": []string{}}}, "parameters": []interface{}{map[string]interface{}{"name": "id", "in": "path", "required": true, "schema": map[string]interface{}{"type": "integer"}}}, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Replacement linked token"}}}},
		"/api/v1/server/about":                             map[string]interface{}{"get": map[string]interface{}{"summary": "About the service", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "About information"}}}},
		"/api/v1/server/help":                              map[string]interface{}{"get": map[string]interface{}{"summary": "API usage guidance", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Help information"}}}},
		"/api/v1/server/privacy":                           map[string]interface{}{"get": map[string]interface{}{"summary": "Privacy information", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Privacy information"}}}},
		"/api/v1/server/terms":                             map[string]interface{}{"get": map[string]interface{}{"summary": "Service terms", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Terms information"}}}},
		"/api/v1/server/dpo":                               map[string]interface{}{"get": map[string]interface{}{"summary": "Data protection officer contact", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "DPO and privacy contact information"}}}},
		"/api/v1/server/help/mcp":                          map[string]interface{}{"get": map[string]interface{}{"operationId": "getMCPHelp", "summary": "MCP connection guide and live tool list", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "MCP help document"}}}},
		"/api/v1/server/mcp":                               map[string]interface{}{"post": mcpOperation},
		"/api/mcp":                                         map[string]interface{}{"post": mcpOperation},
		"/mcp":                                             map[string]interface{}{"post": mcpOperation},
		"/api/v1/server/security":                          map[string]interface{}{"get": map[string]interface{}{"summary": "Security reporting details", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Security contacts, expiry, and policy links"}}}},
		"/api/v1/server/security/policy":                   map[string]interface{}{"get": map[string]interface{}{"summary": "Security disclosure policy", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Disclosure policy Markdown"}}}},
		"/api/v1/server/security/thanks":                   map[string]interface{}{"get": map[string]interface{}{"summary": "Security acknowledgments", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Credited researchers"}}}},
		"/api/v1/server/contact":                           map[string]interface{}{"get": map[string]interface{}{"summary": "Contact report form", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Report types, field definitions, and CAPTCHA challenge"}}}, "post": map[string]interface{}{"summary": "Submit a contact report", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Review confirmation or tracking ID"}, "400": map[string]interface{}{"description": "Field validation failed"}}}},
		"/api/v1/stats":                                    map[string]interface{}{"get": map[string]interface{}{"operationId": "getStats", "summary": "Runtime and collection statistics", "x-mcp-enabled": true, "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Statistics response envelope"}}}},
		"/api/v1/random.txt":                               map[string]interface{}{"get": map[string]interface{}{"summary": "Random quote as plain text", "responses": textResponse}},
		"/api/v1/quotes.txt":                               map[string]interface{}{"get": map[string]interface{}{"summary": "All quotes as plain text", "responses": textResponse}},
		"/api/v1/server/healthz.txt":                       map[string]interface{}{"get": map[string]interface{}{"summary": "Health as plain text", "responses": textResponse}},
		"/api/v1/stats.txt":                                map[string]interface{}{"get": map[string]interface{}{"summary": "Statistics as plain text", "responses": textResponse}},
		"/api/v1/server/graphql":                           map[string]interface{}{"get": graphqlGet, "post": graphqlPost},
		"/api/v1/server/swagger":                           map[string]interface{}{"get": map[string]interface{}{"summary": "OpenAPI JSON", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "OpenAPI 3.1 document"}}}},
		"/api/swagger":                                     map[string]interface{}{"get": map[string]interface{}{"summary": "OpenAPI JSON alias", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "OpenAPI 3.1 document"}}}},
		"/api/healthz":                                     map[string]interface{}{"get": map[string]interface{}{"summary": "Health endpoint alias", "responses": map[string]interface{}{"200": map[string]interface{}{"description": "Health response"}}}},
		"/api/graphql":                                     map[string]interface{}{"get": graphqlGet, "post": graphqlPost},
		"/graphql":                                         map[string]interface{}{"get": graphqlGet, "post": graphqlPost},
	}
	spec := map[string]interface{}{
		"openapi": "3.1.0",
		"info":    map[string]interface{}{"title": "Anime Quotes API", "version": "v1", "description": "Public, read-only anime quote collection."},
		"paths":   paths,
		"components": map[string]interface{}{"securitySchemes": map[string]interface{}{"BearerAuth": map[string]interface{}{"type": "http", "scheme": "bearer"}}, "schemas": map[string]interface{}{
			"Quote":         map[string]interface{}{"type": "object", "required": []string{"anime", "character", "quote"}, "properties": map[string]interface{}{"anime": map[string]interface{}{"type": "string"}, "character": map[string]interface{}{"type": "string"}, "quote": map[string]interface{}{"type": "string"}}},
			"QuoteResponse": map[string]interface{}{"type": "object", "required": []string{"ok", "data"}, "properties": map[string]interface{}{"ok": map[string]interface{}{"type": "boolean", "const": true}, "data": map[string]interface{}{"$ref": "#/components/schemas/Quote"}}},
		}},
	}
	return spec
}

// handleOpenAPI serves the machine-readable contract for the public API.
func (s *Server) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(openAPIDocument())
}

func apiRouteIndex() []string {
	paths, _ := openAPIDocument()["paths"].(map[string]interface{})
	routes := make([]string, 0, len(paths))
	for path := range paths {
		if strings.HasPrefix(path, "/api/v1/") {
			routes = append(routes, path)
		}
	}
	sort.Strings(routes)
	return routes
}

// handleSwaggerUI provides API documentation without a third-party runtime dependency.
func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "REST API documentation", "Description": "REST routes for the read-only Anime Quotes API.",
		"ContentTemplate": "swagger-content", "Page": "api", "Version": Version,
	}
	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering API documentation page: %v", err)
	}
}

// handleGraphQLUI exposes a small, self-hosted query explorer.
func (s *Server) handleGraphQLUI(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "GraphQL explorer", "Description": "Run read-only GraphQL queries against the Anime Quotes API.",
		"ContentTemplate": "graphql-content", "Page": "graphql", "Version": Version,
	}
	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering GraphQL page: %v", err)
	}
}

// handleGraphQL executes read-only queries against the embedded quote collection.
func (s *Server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	var input graphQLRequest
	switch r.Method {
	case http.MethodGet:
		input.Query = r.URL.Query().Get("query")
	case http.MethodPost:
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeProblem(w, http.StatusUnsupportedMediaType, "content type must be application/json")
			return
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeProblem(w, http.StatusBadRequest, "a valid GraphQL request is required")
			return
		}
	default:
		w.Header().Set("Allow", "GET, POST")
		writeProblem(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if strings.TrimSpace(input.Query) == "" {
		writeProblem(w, http.StatusBadRequest, "a GraphQL query is required")
		return
	}
	quoteType := graphql.NewObject(graphql.ObjectConfig{Name: "Quote", Fields: graphql.Fields{
		"anime":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		"character": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		"quote":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
	}})
	queryType := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{
		"randomQuote": &graphql.Field{Type: quoteType, Resolve: func(graphql.ResolveParams) (interface{}, error) {
			quote, ok := s.animeService.GetRandomQuoteRecord()
			if !ok {
				return nil, nil
			}
			return quote, nil
		}},
		"quotes": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(quoteType)), Args: graphql.FieldConfigArgument{
			"q": &graphql.ArgumentConfig{Type: graphql.String}, "anime": &graphql.ArgumentConfig{Type: graphql.String}, "character": &graphql.ArgumentConfig{Type: graphql.String},
		}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			q, _ := p.Args["q"].(string)
			animeName, _ := p.Args["anime"].(string)
			character, _ := p.Args["character"].(string)
			return s.animeService.SearchQuotes(q, animeName, character), nil
		}},
	}})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: queryType})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "GraphQL schema unavailable")
		return
	}
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: input.Query, Context: r.Context()})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
}
