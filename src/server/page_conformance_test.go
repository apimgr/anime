package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestServerRenderedPageConformance(t *testing.T) {
	s := newTestServer(t)
	s.cfg.Server.Pages.Contact.Enabled = true
	s.cfg.Server.Pages.Contact.Captcha = "simple"
	paths := []string{
		"/", "/quotes", "/server/about", "/server/help", "/server/help/mcp",
		"/server/healthz", "/server/preferences", "/server/preferences/tokens",
		"/server/preferences/sync", "/server/contact?type=abuse", "/server/privacy",
		"/server/terms", "/server/dpo", "/server/security", "/server/security/policy",
		"/server/security/thanks", "/server/docs/swagger", "/server/docs/graphql",
		"/server/security/report/missing?token=invalid", "/missing-page",
	}
	trackingID, statusToken, _, err := s.saveSecurityReport(httptest.NewRequest(http.MethodPost, "/server/contact", nil), map[string]string{
		"security_id": "test-security-id", "severity": "low", "component": "api",
		"email": "researcher@example.test", "researcher_gpg": "", "credit_preference": "anonymous",
	})
	if err != nil {
		t.Fatalf("create test security report: %v", err)
	}
	paths = append(paths, "/server/security/report/"+trackingID+"?token="+url.QueryEscape(statusToken))
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			s.router.ServeHTTP(response, request)
			if !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("content type = %q, want HTML", response.Header().Get("Content-Type"))
			}
			root, err := html.Parse(strings.NewReader(response.Body.String()))
			if err != nil {
				t.Fatalf("parse page: %v", err)
			}
			assertPageMarkup(t, s, root, path)
		})
	}
	for _, page := range serverPageLinks {
		request := httptest.NewRequest(http.MethodGet, page.Path, nil)
		response := httptest.NewRecorder()
		s.router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
			t.Errorf("registered page %s returned status=%d content-type=%q", page.Path, response.Code, response.Header().Get("Content-Type"))
		}
		hubRequest := httptest.NewRecorder()
		s.router.ServeHTTP(hubRequest, httptest.NewRequest(http.MethodGet, "/server/"+page.Hub, nil))
		if strings.Count(hubRequest.Body.String(), `href="`+page.Path+`"`) != 1 {
			t.Errorf("%s does not appear exactly once on its %s hub", page.Path, page.Hub)
		}
	}
}

func assertPageMarkup(t *testing.T, s *Server, root *html.Node, requestPath string) {
	t.Helper()
	counts := map[string]int{"main": 0, "h1": 0}
	var body, header *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			counts[node.Data]++
			if node.Data == "body" {
				body = node
			}
			if node.Data == "header" && attr(node, "id") == "main-header" {
				header = node
			}
			for _, attribute := range node.Attr {
				name := strings.ToLower(attribute.Key)
				if name == "style" || strings.HasPrefix(name, "on") {
					t.Errorf("%s contains forbidden %s attribute", requestPath, name)
				}
			}
			switch node.Data {
			case "style":
				t.Errorf("%s contains inline style element", requestPath)
			case "script":
				if attr(node, "type") != "application/ld+json" && attr(node, "src") == "" && strings.TrimSpace(textContent(node)) != "" {
					t.Errorf("%s contains executable inline script", requestPath)
				}
			case "img":
				if _, ok := attrOK(node, "alt"); !ok {
					t.Errorf("%s contains image without alt", requestPath)
				}
			case "table":
				if firstElement(node, "thead") == nil || !hasAncestorClass(node, "table-wrapper") {
					t.Errorf("%s contains table without thead or .table-wrapper", requestPath)
				}
			case "a":
				assertInternalLink(t, s, requestPath, attr(node, "href"))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if counts["main"] != 1 || counts["h1"] != 1 {
		t.Errorf("%s has %d main elements and %d h1 elements; want exactly one each", requestPath, counts["main"], counts["h1"])
	}
	if body != nil {
		var first []string
		for child := body.FirstChild; child != nil && len(first) < 2; child = child.NextSibling {
			if child.Type == html.ElementNode {
				first = append(first, child.Data+"."+attr(child, "class"))
			}
		}
		if strings.Join(first, ",") != "a.skip-link,a.skip-link" {
			t.Errorf("%s starts with %v, want the two skip links", requestPath, first)
		}
	}
	if header != nil {
		var direct []string
		for child := header.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode {
				direct = append(direct, child.Data+"."+attr(child, "class"))
			}
		}
		want := "a.logo,nav.nav-links,label.mobile-menu-toggle,div.header-actions"
		if strings.Join(direct, ",") != want {
			t.Errorf("%s header children = %v, want %s", requestPath, direct, want)
		}
	}
	for _, form := range elements(root, "form") {
		if strings.EqualFold(attr(form, "method"), http.MethodPost) && !formHasInput(form, "csrf_token") {
			t.Errorf("%s contains POST form without csrf_token", requestPath)
		}
		for _, control := range formControls(form) {
			if strings.EqualFold(attr(control, "type"), "hidden") || control.Data == "button" {
				continue
			}
			if !hasLabel(form, control) {
				t.Errorf("%s contains unlabeled %s control %q", requestPath, control.Data, attr(control, "name"))
			}
		}
	}
	if isHubPath(requestPath) && breadcrumbCount(root) != 0 {
		t.Errorf("%s is a hub and must not have a breadcrumb", requestPath)
	}
	if hub := pageHub(requestPath); hub != "" && breadcrumbCount(root) != 1 {
		t.Errorf("%s has %d breadcrumbs; want exactly one", requestPath, breadcrumbCount(root))
	} else if hub != "" && breadcrumbTarget(root) != hub {
		t.Errorf("%s breadcrumb targets %q, want %s", requestPath, breadcrumbTarget(root), hub)
	}
}

func assertInternalLink(t *testing.T, s *Server, from, href string) {
	t.Helper()
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "//") {
		return
	}
	parsed, err := url.Parse(href)
	if err != nil || parsed.IsAbs() || !strings.HasPrefix(parsed.Path, "/") {
		return
	}
	request := httptest.NewRequest(http.MethodGet, parsed.RequestURI(), nil)
	if !s.router.Match(request, &mux.RouteMatch{}) {
		t.Errorf("%s has unresolved internal link %q", from, href)
	}
}

func attr(node *html.Node, key string) string {
	value, _ := attrOK(node, key)
	return value
}

func attrOK(node *html.Node, key string) (string, bool) {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, key) {
			return attribute.Val, true
		}
	}
	return "", false
}

func textContent(node *html.Node) string {
	var output strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			output.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return output.String()
}

func firstElement(node *html.Node, name string) *html.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == name {
			return child
		}
	}
	return nil
}

func hasAncestorClass(node *html.Node, class string) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		for _, value := range strings.Fields(attr(parent, "class")) {
			if value == class {
				return true
			}
		}
	}
	return false
}

func elements(root *html.Node, name string) []*html.Node {
	var found []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == name {
			found = append(found, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func formControls(form *html.Node) []*html.Node {
	var found []*html.Node
	for _, name := range []string{"input", "textarea", "select", "button"} {
		found = append(found, elements(form, name)...)
	}
	return found
}

func formHasInput(form *html.Node, name string) bool {
	for _, input := range elements(form, "input") {
		if attr(input, "name") == name {
			return true
		}
	}
	return false
}

func hasLabel(form, control *html.Node) bool {
	if id := attr(control, "id"); id != "" {
		for _, label := range elements(form, "label") {
			if attr(label, "for") == id {
				return true
			}
		}
	}
	for parent := control.Parent; parent != nil && parent != form; parent = parent.Parent {
		if parent.Type == html.ElementNode && parent.Data == "label" {
			return true
		}
	}
	return false
}

func breadcrumbCount(root *html.Node) int {
	count := 0
	for _, nav := range elements(root, "nav") {
		if strings.EqualFold(attr(nav, "aria-label"), "Breadcrumb") {
			count++
		}
	}
	return count
}

func breadcrumbTarget(root *html.Node) string {
	for _, nav := range elements(root, "nav") {
		if strings.EqualFold(attr(nav, "aria-label"), "Breadcrumb") {
			if link := firstElement(nav, "a"); link != nil {
				return attr(link, "href")
			}
		}
	}
	return ""
}

func pageHub(path string) string {
	if strings.HasPrefix(path, "/server/preferences/") {
		return "/server/preferences"
	}
	if strings.HasPrefix(path, "/server/help/") {
		return "/server/help"
	}
	if strings.HasPrefix(path, "/server/security/") {
		return "/server/security"
	}
	return ""
}

func isHubPath(path string) bool {
	return path == "/server/preferences" || path == "/server/help" || path == "/server/security"
}
