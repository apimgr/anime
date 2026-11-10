package mode

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func resetMode(t *testing.T) {
	t.Helper()
	stateMu.Lock()
	currentMode = AppModeProduction
	debugEnabled = false
	stateMu.Unlock()
	updateProfilingSettings(false)
	t.Cleanup(func() {
		stateMu.Lock()
		currentMode = AppModeProduction
		debugEnabled = false
		stateMu.Unlock()
		updateProfilingSettings(false)
	})
}

func TestParseAppMode(t *testing.T) {
	for _, test := range []struct {
		input string
		want  AppMode
	}{
		{input: "prod", want: AppModeProduction},
		{input: "PRODUCTION", want: AppModeProduction},
		{input: "dev", want: AppModeDevelopment},
		{input: "devel", want: AppModeDevelopment},
		{input: "development", want: AppModeDevelopment},
		{input: "debug", want: AppModeDebug},
	} {
		got, err := ParseAppMode(test.input)
		if err != nil || got != test.want {
			t.Errorf("ParseAppMode(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
	if _, err := ParseAppMode("unknown"); err == nil {
		t.Error("ParseAppMode accepted an unknown mode")
	}
}

func TestConfigureAppModePriorityAndDebugDefault(t *testing.T) {
	resetMode(t)
	t.Setenv("MODE", "production")
	t.Setenv("DEBUG", "")
	if err := ConfigureAppMode("dev", true, true, true); err != nil {
		t.Fatal(err)
	}
	if GetCurrentAppMode() != AppModeDevelopment || !IsDebugEnabled() {
		t.Fatalf("CLI mode/debug priority produced %q, debug=%v", GetCurrentAppMode(), IsDebugEnabled())
	}

	t.Setenv("MODE", "debug")
	t.Setenv("DEBUG", "")
	if err := ConfigureAppMode("", false, false, false); err != nil {
		t.Fatal(err)
	}
	if GetCurrentAppMode() != AppModeDebug || !IsDebugEnabled() {
		t.Fatalf("MODE=debug with an empty DEBUG value produced %q, debug=%v", GetCurrentAppMode(), IsDebugEnabled())
	}

	t.Setenv("DEBUG", "false")
	if err := ConfigureAppMode("", false, false, false); err != nil {
		t.Fatal(err)
	}
	if IsDebugEnabled() {
		t.Fatal("DEBUG=false did not disable debug diagnostics")
	}

	t.Setenv("DEBUG", "yes")
	if err := ConfigureAppMode("", false, false, true); err != nil {
		t.Fatal(err)
	}
	if IsDebugEnabled() {
		t.Fatal("explicit false CLI debug flag did not override DEBUG")
	}
}

func TestConfigureAppModeRejectsInvalidEnvironment(t *testing.T) {
	resetMode(t)
	t.Setenv("MODE", "unknown")
	if err := ConfigureAppMode("", false, false, false); err == nil {
		t.Fatal("invalid MODE was accepted")
	}
	t.Setenv("MODE", "production")
	t.Setenv("DEBUG", "maybe")
	if err := ConfigureAppMode("", false, false, false); err == nil {
		t.Fatal("invalid DEBUG was accepted")
	}
}

func TestAppModeBehavior(t *testing.T) {
	resetMode(t)
	if !IsAppModeProd() || ShouldShowDebugEndpoints() {
		t.Fatal("default mode is not secure production")
	}
	if err := SetAppMode("development"); err != nil {
		t.Fatal(err)
	}
	if !IsAppModeDev() || ShouldShowDebugEndpoints() {
		t.Fatal("development mode enabled debug endpoints or was not selected")
	}
	if err := SetAppMode("debug"); err != nil {
		t.Fatal(err)
	}
	if !IsAppModeDebug() || !ShouldShowDebugEndpoints() {
		t.Fatal("debug mode did not select debug mode and enable diagnostics")
	}
}

func TestGetClientErrorDetail(t *testing.T) {
	resetMode(t)
	err := errors.New("internal failure")
	if got := GetClientErrorDetail(err); got != "An internal error occurred" {
		t.Fatalf("production error detail = %q", got)
	}
	if err := SetAppMode("development"); err != nil {
		t.Fatal(err)
	}
	if got := GetClientErrorDetail(err); got != err.Error() {
		t.Fatalf("development error detail = %q", got)
	}
}

func TestHTTPStaticCacheHeaders(t *testing.T) {
	resetMode(t)
	if got := GetHTTPStaticCacheHeaders()["Cache-Control"]; got != "public, max-age=31536000, immutable" {
		t.Fatalf("production cache header = %q", got)
	}
	if err := SetAppMode("development"); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	ApplyHTTPStaticCacheHeaders(response)
	if got := response.Header().Get("Cache-Control"); got != "no-cache, no-store, must-revalidate" {
		t.Fatalf("development cache header = %q", got)
	}
}

func TestAppModeDefaults(t *testing.T) {
	resetMode(t)
	if GetAppModeLogLevel() != "info" || !ShouldAppModeCacheTemplates() || !ShouldAppModeCacheStaticFiles() || ShouldAppModeAutoReload() {
		t.Fatal("unexpected production defaults")
	}
	if err := SetAppMode("development"); err != nil {
		t.Fatal(err)
	}
	if GetAppModeLogLevel() != "debug" || ShouldAppModeCacheTemplates() || ShouldAppModeCacheStaticFiles() || !ShouldAppModeAutoReload() {
		t.Fatal("unexpected development defaults")
	}
}
