//go:build e2e

package e2e

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

func TestSSRCorePages(t *testing.T) {
	for _, path := range []string{"/", "/quotes", "/server/about", "/server/help", "/server/privacy", "/server/terms", "/server/docs/swagger", "/server/docs/graphql"} {
		t.Run(path, func(t *testing.T) {
			response, err := http.Get(baseURL() + path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", response.StatusCode)
			}
			if !strings.Contains(response.Header.Get("Content-Type"), "text/html") {
				t.Fatalf("unexpected content type %q", response.Header.Get("Content-Type"))
			}
			if path == "/" || path == "/quotes" {
				body, readErr := io.ReadAll(response.Body)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !strings.Contains(strings.ToLower(string(body)), "anime") {
					t.Fatal("SSR response did not contain real page content")
				}
			}
		})
	}
}

func TestNoJavaScriptQuoteSearch(t *testing.T) {
	ctx, cancel := browserContext(t)
	defer cancel()
	var html string
	if err := chromedp.Run(ctx,
		emulation.SetScriptExecutionDisabled(true),
		chromedp.Navigate(baseURL()+"/quotes?q=naruto"),
		chromedp.OuterHTML("html", &html),
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(html), "matching quotes") || !strings.Contains(strings.ToLower(html), "naruto") {
		t.Fatal("search page did not render results without JavaScript")
	}
}

func TestFullBrowserNavigation(t *testing.T) {
	ctx, cancel := browserContext(t)
	defer cancel()
	var title, html string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL()+"/"),
		chromedp.Title(&title),
		chromedp.Click(`a[href="/quotes"]`, chromedp.ByQuery),
		chromedp.OuterHTML("html", &html),
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(title), "anime") || !strings.Contains(strings.ToLower(html), "matching quotes") {
		t.Fatal("browser navigation did not render the quote listing")
	}
}

func browserContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	allocator, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), chromeURL())
	ctx, cancelContext := chromedp.NewContext(allocator)
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	return ctx, func() { cancelTimeout(); cancelContext(); cancelAllocator() }
}

func baseURL() string {
	if value := strings.TrimRight(os.Getenv("E2E_BASE_URL"), "/"); value != "" {
		return value
	}
	return "http://127.0.0.1:64580"
}

func chromeURL() string {
	if value := os.Getenv("E2E_CHROME_URL"); value != "" {
		return value
	}
	return "ws://127.0.0.1:9222"
}
