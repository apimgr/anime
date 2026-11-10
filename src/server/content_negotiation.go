package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"mime"
	"net/http"
	"sort"
	"strings"

	htmltoken "golang.org/x/net/html"
)

func contentNegotiationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !prefersPlainText(r.Header.Get("Accept")) {
			next.ServeHTTP(w, r)
			return
		}

		recorder := newResponseRecorder()
		next.ServeHTTP(recorder, r)
		for key, values := range recorder.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.Header().Add("Vary", "Accept")
		if recorder.status != http.StatusOK {
			w.WriteHeader(recorder.status)
			_, _ = w.Write(recorder.body.Bytes())
			return
		}

		body := recorder.body.Bytes()
		contentType := recorder.header.Get("Content-Type")
		if contentType == "" && len(body) > 0 {
			contentType = http.DetectContentType(body)
		}
		switch {
		case strings.Contains(contentType, "application/json"):
			body = jsonAsText(body)
		case strings.Contains(contentType, "text/html"):
			body = htmlAsText(body)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(recorder.status)
		_, _ = w.Write(body)
	})
}

type responseRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header), status: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header { return r.header }
func (r *responseRecorder) WriteHeader(status int) {
	if r.status == http.StatusOK {
		r.status = status
	}
}
func (r *responseRecorder) Write(body []byte) (int, error) { return r.body.Write(body) }

func prefersPlainText(header string) bool {
	plainQuality, htmlQuality := 0.0, 0.0
	for _, part := range strings.Split(header, ",") {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		quality := 1.0
		if value, ok := params["q"]; ok {
			if _, err := fmt.Sscanf(value, "%f", &quality); err != nil || quality < 0 || quality > 1 {
				quality = 0
			}
		}
		switch mediaType {
		case "text/plain":
			plainQuality = quality
		case "text/html":
			htmlQuality = quality
		}
	}
	return plainQuality > 0 && plainQuality >= htmlQuality
}

func jsonAsText(body []byte) []byte {
	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return bytes.TrimSpace(body)
	}
	if envelope, ok := value.(map[string]interface{}); ok {
		if data, exists := envelope["data"]; exists {
			value = data
		}
	}
	var output strings.Builder
	appendPlainValue(&output, value, 0, "")
	return []byte(strings.TrimSpace(output.String()))
}

func appendPlainValue(output *strings.Builder, value interface{}, depth int, prefix string) {
	indent := strings.Repeat("  ", depth)
	switch typed := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			appendPlainValue(output, typed[key], depth+1, indent+key+": ")
		}
	case []interface{}:
		for _, item := range typed {
			appendPlainValue(output, item, depth+1, indent+"- ")
		}
	default:
		output.WriteString(prefix)
		output.WriteString(fmt.Sprint(typed))
		output.WriteByte('\n')
	}
}

func htmlAsText(body []byte) []byte {
	tokenizer := htmltoken.NewTokenizer(bytes.NewReader(body))
	var output strings.Builder
	skip := 0
	for {
		tokenType := tokenizer.Next()
		if tokenType == htmltoken.ErrorToken {
			break
		}
		token := tokenizer.Token()
		switch tokenType {
		case htmltoken.StartTagToken, htmltoken.SelfClosingTagToken:
			name := strings.ToLower(token.Data)
			if name == "script" || name == "style" {
				skip++
			}
			if skip == 0 && isBlockTag(name) {
				output.WriteByte('\n')
			}
		case htmltoken.EndTagToken:
			name := strings.ToLower(token.Data)
			if (name == "script" || name == "style") && skip > 0 {
				skip--
			}
			if skip == 0 && isBlockTag(name) {
				output.WriteByte('\n')
			}
		case htmltoken.TextToken:
			if skip == 0 {
				output.WriteString(html.UnescapeString(string(token.Data)))
			}
		}
	}
	lines := strings.Split(output.String(), "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			clean = append(clean, line)
		}
	}
	return []byte(strings.Join(clean, "\n") + "\n")
}

func isBlockTag(name string) bool {
	switch name {
	case "address", "article", "aside", "blockquote", "br", "dd", "div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "tr", "ul":
		return true
	default:
		return false
	}
}
