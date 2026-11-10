package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/anime/src/updater"
)

type cliVersion struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

type autodiscoverResponse struct {
	Server struct {
		Version    string `json:"version"`
		APIVersion string `json:"api_version"`
	} `json:"server"`
	CLIVersions   map[string]cliVersion `json:"cli_versions"`
	CLIMinVersion string                `json:"cli_min_version,omitempty"`
}

type cliUpdateCache struct {
	mu       sync.Mutex
	branch   string
	expires  time.Time
	release  *updater.Release
	versions map[string]cliVersion
}

var cliPlatforms = [][2]string{
	{"linux", "amd64"}, {"linux", "arm64"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
	{"windows", "amd64"}, {"windows", "arm64"},
	{"freebsd", "amd64"}, {"freebsd", "arm64"},
}

func (s *Server) handleAutodiscover(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	_, versions, err := s.getCLIUpdate(ctx)
	if err != nil {
		http.Error(w, "client update metadata is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	response := autodiscoverResponse{CLIVersions: versions}
	response.Server.Version = Version
	response.Server.APIVersion = "v1"
	if s.cfg != nil {
		response.CLIMinVersion = strings.TrimSpace(s.cfg.Server.Update.CLIMinVersion)
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}

func (s *Server) handleCLIBinary(w http.ResponseWriter, r *http.Request) {
	assetName := strings.TrimPrefix(r.URL.Path, "/cli/binaries/")
	if assetName == "" || strings.Contains(assetName, "/") {
		http.NotFound(w, r)
		return
	}
	goos, goarch, ok := parseCLIBinaryName(assetName)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	release, _, err := s.getCLIUpdate(ctx)
	if err != nil {
		http.Error(w, "client binary is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if release == nil || !hasReleaseAsset(release, cliBinaryName(goos, goarch)) {
		http.NotFound(w, r)
		return
	}
	assetURL, err := s.updateClient.AssetURL(release, assetName)
	if err != nil {
		http.Error(w, "client binary is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.Redirect(w, r, assetURL, http.StatusFound)
}

func (s *Server) getCLIUpdate(ctx context.Context) (*updater.Release, map[string]cliVersion, error) {
	cache := s.cliUpdates
	if cache == nil {
		cache = &cliUpdateCache{}
		s.cliUpdates = cache
	}
	branch := updateChannel(s)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.branch == branch && time.Now().Before(cache.expires) {
		return cache.release, cache.versions, nil
	}
	release, err := s.updateClient.Check(ctx, "0.0.0", branch, 0)
	if err != nil {
		return nil, nil, err
	}
	versions := make(map[string]cliVersion)
	if release != nil {
		checksums, err := s.updateClient.AssetChecksums(ctx, release)
		if err != nil {
			return nil, nil, err
		}
		for _, platform := range cliPlatforms {
			assetName := cliBinaryName(platform[0], platform[1])
			if !hasReleaseAsset(release, assetName) {
				continue
			}
			checksum, ok := checksums[assetName]
			if !ok {
				return nil, nil, fmt.Errorf("release checksum is missing for %q", assetName)
			}
			versions[platform[0]+"-"+platform[1]] = cliVersion{
				Version: strings.TrimPrefix(release.TagName, "v"),
				SHA256:  checksum,
			}
		}
	}
	cache.branch = branch
	cache.release = release
	cache.versions = versions
	cache.expires = time.Now().Add(5 * time.Minute)
	return release, versions, nil
}

func updateChannel(s *Server) string {
	if s.cfg == nil || strings.TrimSpace(s.cfg.Server.Update.Branch) == "" {
		return "stable"
	}
	return s.cfg.Server.Update.Branch
}

func cliBinaryName(goos, goarch string) string {
	name := fmt.Sprintf("anime-cli-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func parseCLIBinaryName(name string) (string, string, bool) {
	for _, platform := range cliPlatforms {
		if cliBinaryName(platform[0], platform[1]) == name {
			return platform[0], platform[1], true
		}
	}
	return "", "", false
}

func hasReleaseAsset(release *updater.Release, name string) bool {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return true
		}
	}
	return false
}
