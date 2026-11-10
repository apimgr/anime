package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeBranchAndBinaryName(t *testing.T) {
	for input, expected := range map[string]string{"": "stable", "stable": "stable", "beta": "beta", "daily": "devel", "rolling": "devel", "dev": "devel", "development": "devel"} {
		got, err := NormalizeBranch(input)
		if err != nil || got != expected {
			t.Errorf("NormalizeBranch(%q)=%q err=%v", input, got, err)
		}
	}
	if _, err := NormalizeBranch("unknown"); err == nil {
		t.Fatal("unknown channel succeeded")
	}
	if BinaryName("windows", "arm64") != "anime-windows-arm64.exe" {
		t.Fatal("Windows asset name is incorrect")
	}
	if BinaryName("linux", "amd64") != "anime-linux-amd64" {
		t.Fatal("Linux asset name is incorrect")
	}
}

func TestVersionComparisonAndChannelMatching(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{"v1.2.0", "1.1.9", 1},
		{"1.2.0-beta", "1.2.0", -1},
		{"1.2.0", "1.2.0-beta", 1},
		{"1.2.0", "v1.2.0", 0},
		{"1.2.0", "1.2.1", -1},
	} {
		if got := compareVersions(test.left, test.right); got != test.want {
			t.Errorf("compareVersions(%q,%q)=%d want %d", test.left, test.right, got, test.want)
		}
	}
	if !matchesBranch(Release{TagName: "v1.0.0"}, "beta") || !matchesBranch(Release{TagName: "v1.0.0-beta", Prerelease: true}, "devel") || matchesBranch(Release{TagName: "devel", Prerelease: true}, "beta") {
		t.Fatal("update channel matching failed")
	}
}

func TestCheckForUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/apimgr/anime/releases/latest":
			_, _ = fmt.Fprint(w, `{"tag_name":"v1.3.0","published_at":"2026-01-01T00:00:00Z","assets":[]}`)
		case "/repos/apimgr/anime/releases":
			_, _ = fmt.Fprint(w, `[{"tag_name":"v1.4.0-beta","prerelease":true,"published_at":"2026-02-01T00:00:00Z"},{"tag_name":"v1.3.0","published_at":"2026-01-01T00:00:00Z"},{"tag_name":"devel","prerelease":true,"published_at":"2026-03-01T00:00:00Z"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := &Client{APIBase: server.URL + "/repos/apimgr/anime", HTTP: server.Client(), AllowHTTP: true}
	stable, err := client.Check(context.Background(), "1.2.0", "stable", 0)
	if err != nil || stable == nil || stable.TagName != "v1.3.0" {
		t.Fatalf("stable release=%#v err=%v", stable, err)
	}
	beta, err := client.Check(context.Background(), "1.2.0", "beta", 0)
	if err != nil || beta == nil || beta.TagName != "v1.4.0-beta" {
		t.Fatalf("beta release=%#v err=%v", beta, err)
	}
	devel, err := client.Check(context.Background(), "1.2.0", "daily", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).Unix())
	if err != nil || devel == nil || devel.TagName != "v1.4.0-beta" {
		t.Fatalf("devel release=%#v err=%v", devel, err)
	}
	none, err := client.Check(context.Background(), "1.3.0", "stable", 0)
	if err != nil || none != nil {
		t.Fatalf("current release=%#v err=%v", none, err)
	}
}

func TestDownloadChecksumAndInstall(t *testing.T) {
	binary := []byte("verified new executable")
	hash := sha256.Sum256(binary)
	checksum := hex.EncodeToString(hash[:]) + "  anime-linux-amd64\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/binary":
			_, _ = w.Write(binary)
		case "/checksums":
			_, _ = w.Write([]byte(checksum))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := &Client{HTTP: server.Client(), TempRoot: t.TempDir(), AllowHTTP: true}
	release := &Release{Assets: []Asset{{Name: "anime-linux-amd64", BrowserDownloadURL: server.URL + "/binary"}, {Name: "sha256.txt", BrowserDownloadURL: server.URL + "/checksums"}}}
	verified, err := client.DownloadAndVerify(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(verified, "verified.tmp") {
		t.Fatalf("verified update path=%q", verified)
	}
	if err := VerifyChecksum(verified, hex.EncodeToString(hash[:])); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(t.TempDir(), "anime")
	if err := os.WriteFile(currentPath, []byte("old executable"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := client.Install(context.Background(), release, currentPath); err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile(currentPath)
	if err != nil || string(installed) != string(binary) {
		t.Fatalf("installed binary=%q err=%v", installed, err)
	}
	info, err := os.Stat(currentPath)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("installed mode=%v err=%v", info, err)
	}
}

func TestDownloadRejectsInvalidAssetsAndChecksums(t *testing.T) {
	client := NewClient()
	if _, err := client.DownloadAndVerify(context.Background(), nil); err == nil {
		t.Fatal("nil release accepted")
	}
	if _, err := client.DownloadAndVerify(context.Background(), &Release{}); err == nil {
		t.Fatal("release without assets accepted")
	}
	if _, err := checksumForFile(filepath.Join(t.TempDir(), "missing"), "anime-linux-amd64"); err == nil {
		t.Fatal("missing checksum file accepted")
	}
	file := filepath.Join(t.TempDir(), "sha256.txt")
	if err := os.WriteFile(file, []byte("bad-hash  anime-linux-amd64\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := checksumForFile(file, "anime-linux-amd64"); err == nil {
		t.Fatal("malformed checksum accepted")
	}
	if err := VerifyChecksum(file, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong checksum accepted")
	}
}
