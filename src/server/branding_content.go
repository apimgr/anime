package server

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	brandingVariable = regexp.MustCompile(`\{(current_year|project_name|project_org|project_version|build_datetime|onion_address|i2p_address)\}`)
	brandingWarnings sync.Map
)

func (s *Server) operatorHTML(r *http.Request, name string) template.HTML {
	entry, ok := s.cfg.Server.Branding.Content[name]
	if !ok {
		return ""
	}
	if name != "header" && name != "about" && name != "privacy" && name != "terms" && name != "footer" {
		return ""
	}
	format := strings.ToLower(strings.TrimSpace(entry.Format))
	if format == "" {
		format = "md"
	}
	if (name == "header" || name == "footer") && format != "html" {
		warnBrandingContent(name, "header and footer content only support html format")
		return ""
	}
	if format != "html" && format != "md" && format != "txt" {
		warnBrandingContent(name, "unsupported content format")
		return ""
	}
	body := entry.Body.Resolve(s.resolvedLanguage(r))
	if len(body) > 256*1024 {
		warnBrandingContent(name, "content exceeds 256 KiB")
		return ""
	}
	if strings.TrimSpace(body) == "" {
		return ""
	}
	body = brandingVariable.ReplaceAllStringFunc(body, func(token string) string {
		variable := strings.Trim(token, "{}")
		return html.EscapeString(s.brandingVariable(r, variable))
	})
	switch format {
	case "md":
		var rendered bytes.Buffer
		if err := goldmark.Convert([]byte(body), &rendered); err != nil {
			warnBrandingContent(name, "markdown rendering failed")
			return ""
		}
		body = rendered.String()
	case "txt":
		paragraphs := strings.Split(strings.TrimSpace(body), "\n\n")
		for index := range paragraphs {
			paragraphs[index] = "<p>" + strings.ReplaceAll(html.EscapeString(strings.TrimSpace(paragraphs[index])), "\n", "<br>") + "</p>"
		}
		body = strings.Join(paragraphs, "\n")
	case "html":
	}
	policy := brandingContentPolicy(name)
	sanitized := policy.Sanitize(body)
	if strings.Contains(strings.ToLower(body), "<script") && !strings.Contains(strings.ToLower(sanitized), "<script") {
		warnBrandingContent(name, "unsafe HTML was removed")
	}
	sanitized = stripContactFormLinks(sanitized)
	return template.HTML(sanitized)
}

func brandingContentPolicy(name string) *bluemonday.Policy {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "span", "div", "strong", "b", "em", "i", "u", "s", "small", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "a", "img")
	policy.AllowAttrs("href", "title", "target", "rel").OnElements("a")
	policy.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
	policy.AllowAttrs("class", "id").Globally()
	policy.AllowURLSchemes("https", "mailto")
	policy.AllowRelativeURLs(true)
	policy.RequireParseableURLs(true)
	policy.RequireNoReferrerOnLinks(true)
	policy.AllowDataURIImages()
	if name != "header" && name != "footer" {
		policy.AllowElements("table", "thead", "tbody", "tr", "th", "td", "pre", "code", "blockquote", "hr", "dl", "dt", "dd", "sup", "sub")
	}
	return policy
}

func stripContactFormLinks(fragment string) string {
	context := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := xhtml.ParseFragment(strings.NewReader(fragment), context)
	if err != nil {
		return ""
	}
	var clean func(*xhtml.Node)
	clean = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "a" {
			attributes := node.Attr[:0]
			for _, attribute := range node.Attr {
				if attribute.Key == "href" {
					parsed, parseErr := url.Parse(attribute.Val)
					if parseErr == nil && parsed.Path == "/server/contact" {
						continue
					}
				}
				attributes = append(attributes, attribute)
			}
			node.Attr = attributes
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			clean(child)
		}
	}
	var output bytes.Buffer
	for _, node := range nodes {
		clean(node)
		if err := xhtml.Render(&output, node); err != nil {
			return ""
		}
	}
	return output.String()
}

func (s *Server) brandingVariable(r *http.Request, name string) string {
	switch name {
	case "current_year":
		return fmt.Sprint(time.Now().Year())
	case "project_name":
		return "anime"
	case "project_org":
		return "apimgr"
	case "project_version":
		return Version
	case "build_datetime":
		if parsed, err := time.Parse(time.RFC3339, BuildDate); err == nil {
			return parsed.Local().Format("January 02, 2006 at 03:04:05 PM MST")
		}
		return BuildDate
	case "onion_address":
		_, running, _, host := torHealth()
		if running && host != "" {
			return host
		}
	}
	return ""
}

func (s *Server) operatorContentConfigured(r *http.Request, name string) bool {
	entry, exists := s.cfg.Server.Branding.Content[name]
	if !exists {
		return false
	}
	format := strings.ToLower(strings.TrimSpace(entry.Format))
	if format == "" {
		format = "md"
	}
	if (name == "header" || name == "footer") && format != "html" || format != "html" && format != "md" && format != "txt" {
		return false
	}
	body, exists := entry.Body[s.resolvedLanguage(r)]
	if !exists {
		body, exists = entry.Body["default"]
	}
	return exists && len(body) <= 256*1024
}

func warnBrandingContent(name, reason string) {
	if _, loaded := brandingWarnings.LoadOrStore(name+":"+reason, struct{}{}); !loaded {
		log.Printf("branding.content.%s ignored: %s", name, reason)
	}
}
