package mode

import (
	"net/http/httptest"
	"testing"
)

func resetMode(t *testing.T) {
	t.Helper()
	mu.Lock()
	currentMode = Production
	mu.Unlock()
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Mode
		wantErr bool
	}{
		{"dev", "dev", Development, false},
		{"development", "development", Development, false},
		{"prod", "prod", Production, false},
		{"production", "production", Production, false},
		{"uppercase", "PRODUCTION", Production, false},
		{"whitespace", "  dev  ", Development, false},
		{"invalid", "bogus", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseMode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSetAndGet(t *testing.T) {
	resetMode(t)
	defer resetMode(t)

	if err := Set("development"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if Get() != Development {
		t.Errorf("Get() = %q, want %q", Get(), Development)
	}

	if err := Set("invalid"); err == nil {
		t.Error("expected error for invalid mode")
	}
	if Get() != Development {
		t.Errorf("Get() should remain unchanged after failed Set, got %q", Get())
	}
}

func TestIsDevelopmentIsProduction(t *testing.T) {
	resetMode(t)
	defer resetMode(t)

	if !IsProduction() || IsDevelopment() {
		t.Error("expected default mode to be production")
	}

	if err := Set("development"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if !IsDevelopment() || IsProduction() {
		t.Error("expected mode to be development after Set")
	}
}

func TestShouldShowDebugEndpoints(t *testing.T) {
	resetMode(t)
	defer resetMode(t)

	if ShouldShowDebugEndpoints() {
		t.Error("expected debug endpoints disabled in production")
	}
	Set("development")
	if !ShouldShowDebugEndpoints() {
		t.Error("expected debug endpoints enabled in development")
	}
}

func TestGetErrorDetail(t *testing.T) {
	resetMode(t)
	defer resetMode(t)

	if GetErrorDetail(nil) != "" {
		t.Error("expected empty string for nil error")
	}

	err := errString("boom")
	if got := GetErrorDetail(err); got != "An internal error occurred" {
		t.Errorf("expected generic message in production, got %q", got)
	}

	Set("development")
	if got := GetErrorDetail(err); got != "boom" {
		t.Errorf("expected full error message in development, got %q", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestGetCacheHeadersAndApply(t *testing.T) {
	resetMode(t)
	defer resetMode(t)

	headers := GetCacheHeaders()
	if headers["Cache-Control"] != "public, max-age=31536000, immutable" {
		t.Errorf("unexpected production cache header: %v", headers)
	}

	Set("development")
	headers = GetCacheHeaders()
	if headers["Cache-Control"] != "no-cache, no-store, must-revalidate" {
		t.Errorf("unexpected development cache header: %v", headers)
	}

	w := httptest.NewRecorder()
	ApplyCacheHeaders(w)
	if w.Header().Get("Pragma") != "no-cache" {
		t.Error("expected Pragma header to be applied")
	}
}

func TestGetLogLevel(t *testing.T) {
	resetMode(t)
	defer resetMode(t)

	if GetLogLevel() != "info" {
		t.Errorf("expected info in production, got %q", GetLogLevel())
	}
	Set("development")
	if GetLogLevel() != "debug" {
		t.Errorf("expected debug in development, got %q", GetLogLevel())
	}
}

func TestCacheAndReloadFlags(t *testing.T) {
	resetMode(t)
	defer resetMode(t)

	if !ShouldCacheTemplates() || !ShouldCacheStaticFiles() || ShouldAutoReload() || ShouldEnableProfiling() {
		t.Error("unexpected production flag values")
	}

	Set("development")
	if ShouldCacheTemplates() || ShouldCacheStaticFiles() || !ShouldAutoReload() || !ShouldEnableProfiling() {
		t.Error("unexpected development flag values")
	}
}

func TestModeString(t *testing.T) {
	if Production.String() != "production" {
		t.Errorf("expected %q, got %q", "production", Production.String())
	}
	if Development.String() != "development" {
		t.Errorf("expected %q, got %q", "development", Development.String())
	}
}
