package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
)

type graphQLRequest struct {
	Query string `json:"query"`
}

// handleOpenAPI serves the machine-readable contract for the public API.
func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = io.WriteString(w, `{"openapi":"3.1.0","info":{"title":"Anime Quotes API","version":"v1","description":"Public, read-only anime quote collection."},"paths":{"/api/v1":{"get":{"summary":"API index","responses":{"200":{"description":"Route index"}}}},"/api/v1/random":{"get":{"summary":"Get a random quote","responses":{"200":{"description":"Quote"}}}},"/api/v1/quotes":{"get":{"summary":"List and search quotes","parameters":[{"name":"q","in":"query","schema":{"type":"string"}},{"name":"anime","in":"query","schema":{"type":"string"}},{"name":"character","in":"query","schema":{"type":"string"}}],"responses":{"200":{"description":"Matching quotes"}}}},"/api/v1/server/healthz":{"get":{"summary":"Health status","responses":{"200":{"description":"Healthy"}}}},"/api/v1/server/graphql":{"get":{"summary":"Execute read-only GraphQL query","parameters":[{"name":"query","in":"query","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"GraphQL result"}}},"post":{"summary":"Execute read-only GraphQL query","responses":{"200":{"description":"GraphQL result"}}}}}}`)
}

// handleSwaggerUI provides API documentation without a third-party runtime dependency.
func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Anime API documentation</title><main><p><a href="/">Anime Quotes</a></p><h1>API documentation</h1><p>Machine-readable OpenAPI 3.1 specification: <a href="/api/v1/server/swagger">JSON</a>.</p><h2>GET /api/v1/random</h2><p>Returns one random quote.</p><h2>GET /api/v1/quotes</h2><p>Returns all quotes; optional case-insensitive <code>q</code>, <code>anime</code>, and <code>character</code> filters.</p><h2>GraphQL</h2><p><a href="/server/docs/graphql">Open GraphQL explorer</a>.</p></main></html>`)
}

// handleGraphQLUI exposes a small, self-hosted query explorer.
func (s *Server) handleGraphQLUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Anime GraphQL</title><main><p><a href="/">Anime Quotes</a></p><h1>GraphQL explorer</h1><form id="query-form" method="get" action="/api/graphql"><label for="query">GraphQL query</label><textarea id="query" name="query" rows="8" cols="80">{ randomQuote { anime character quote } }</textarea><button>Run query</button></form><pre id="result" aria-live="polite"></pre><p>POST JSON to <code>/api/v1/server/graphql</code> or <code>/api/graphql</code>.</p><script>document.getElementById("query-form").addEventListener("submit", async event => { event.preventDefault(); const response = await fetch("/api/graphql", {method:"POST", headers:{"Content-Type":"application/json"}, body:JSON.stringify({query:document.getElementById("query").value})}); document.getElementById("result").textContent = JSON.stringify(await response.json(), null, 2); });</script></main></html>`)
}

// handleGraphQL executes read-only queries against the embedded quote collection.
func (s *Server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	var input graphQLRequest
	if r.Method == http.MethodGet {
		input.Query = r.URL.Query().Get("query")
	} else if r.Method == http.MethodPost {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "a valid GraphQL request is required", http.StatusBadRequest)
			return
		}
	} else {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.TrimSpace(input.Query) == "" {
		http.Error(w, "a GraphQL query is required", http.StatusBadRequest)
		return
	}
	quoteType := graphql.NewObject(graphql.ObjectConfig{Name: "Quote", Fields: graphql.Fields{
		"anime": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		"character": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		"quote": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
	}})
	queryType := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{
		"randomQuote": &graphql.Field{Type: quoteType, Resolve: func(graphql.ResolveParams) (interface{}, error) { quote, ok := s.animeService.GetRandomQuoteRecord(); if !ok { return nil, nil }; return quote, nil }},
		"quotes": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(quoteType)), Args: graphql.FieldConfigArgument{
			"q": &graphql.ArgumentConfig{Type: graphql.String}, "anime": &graphql.ArgumentConfig{Type: graphql.String}, "character": &graphql.ArgumentConfig{Type: graphql.String},
		}, Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			q, _ := p.Args["q"].(string); animeName, _ := p.Args["anime"].(string); character, _ := p.Args["character"].(string)
			return s.animeService.SearchQuotes(q, animeName, character), nil
		}},
	}})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: queryType})
	if err != nil { http.Error(w, "GraphQL schema unavailable", http.StatusInternalServerError); return }
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: input.Query, Context: r.Context()})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil { return }
}

