package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	projectOrg  = "apimgr"
	projectName = "anime"
	maxResponse = 32 << 20
	maxBinary   = 512 << 20
)

type Release struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Client struct {
	APIBase   string
	HTTP      *http.Client
	TempRoot  string
	AllowHTTP bool
}

func NewClient() *Client {
	return &Client{
		APIBase: "https://api.github.com/repos/apimgr/anime",
		HTTP:    &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func NormalizeBranch(branch string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(branch)) {
	case "", "stable":
		return "stable", nil
	case "beta":
		return "beta", nil
	case "devel", "daily", "rolling", "dev", "development":
		return "devel", nil
	default:
		return "", fmt.Errorf("unsupported update channel %q", branch)
	}
}

func (c *Client) Check(ctx context.Context, currentVersion, branch string, buildEpoch int64) (*Release, error) {
	channel, err := NormalizeBranch(branch)
	if err != nil {
		return nil, err
	}
	base, err := c.apiBase()
	if err != nil {
		return nil, err
	}
	if channel == "stable" {
		var release Release
		status, err := c.getJSON(ctx, strings.TrimRight(base, "/")+"/releases/latest", &release)
		if err != nil {
			return nil, err
		}
		if status == http.StatusNotFound || release.Draft || release.TagName == "" {
			return nil, nil
		}
		if compareVersions(release.TagName, currentVersion) <= 0 {
			return nil, nil
		}
		return &release, nil
	}
	var releases []Release
	status, err := c.getJSON(ctx, strings.TrimRight(base, "/")+"/releases?per_page=100", &releases)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, nil
	}
	for index := range releases {
		release := releases[index]
		if release.Draft || !matchesBranch(release, channel) {
			continue
		}
		if release.TagName == "devel" {
			if channel == "devel" && release.PublishedAt.Unix() > buildEpoch {
				return &release, nil
			}
			continue
		}
		if compareVersions(release.TagName, currentVersion) > 0 {
			return &release, nil
		}
	}
	return nil, nil
}

func (c *Client) AssetChecksum(ctx context.Context, release *Release, assetName string) (string, error) {
	checksums, err := c.AssetChecksums(ctx, release)
	if err != nil {
		return "", err
	}
	checksum, ok := checksums[assetName]
	if !ok {
		return "", fmt.Errorf("release checksums have no valid entry for %q", assetName)
	}
	return checksum, nil
}

func (c *Client) AssetChecksums(ctx context.Context, release *Release) (map[string]string, error) {
	checksumURL, err := releaseAssetURL(release, "sha256.txt")
	if err != nil {
		return nil, err
	}
	if err := c.validateDownloadURL(checksumURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create checksum request: %w", err)
	}
	request.Header.Set("User-Agent", projectName+"-updater")
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("download release checksums: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release checksums returned HTTP %d", response.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil {
		return nil, fmt.Errorf("read release checksums: %w", err)
	}
	if len(contents) > 1<<20 {
		return nil, errors.New("release checksums exceed size limit")
	}
	checksums := make(map[string]string)
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(fields[0]); err == nil {
			checksums[fields[1]] = strings.ToLower(fields[0])
		}
	}
	return checksums, nil
}

func (c *Client) AssetURL(release *Release, assetName string) (string, error) {
	assetURL, err := releaseAssetURL(release, assetName)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(assetURL)
	if err != nil || parsed.User != nil || (parsed.Scheme != "https" && !(c.AllowHTTP && parsed.Scheme == "http")) {
		return "", errors.New("release asset URL must use the official HTTPS GitHub host")
	}
	if parsed.Host != "github.com" && !(c.AllowHTTP && (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost")) {
		return "", errors.New("release asset URL must use the official HTTPS GitHub host")
	}
	return assetURL, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, target interface{}) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("create update request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", projectName+"-updater")
	response, err := c.httpClient().Do(request)
	if err != nil {
		return 0, fmt.Errorf("request release metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return response.StatusCode, nil
	}
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, fmt.Errorf("release service returned HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponse))
	if err := decoder.Decode(target); err != nil {
		return response.StatusCode, fmt.Errorf("decode release metadata: %w", err)
	}
	return response.StatusCode, nil
}

func (c *Client) DownloadAndVerify(ctx context.Context, release *Release) (string, error) {
	if release == nil {
		return "", errors.New("release is required")
	}
	assetName := BinaryName(runtime.GOOS, runtime.GOARCH)
	binaryURL, err := releaseAssetURL(release, assetName)
	if err != nil {
		return "", err
	}
	checksumURL, err := releaseAssetURL(release, "sha256.txt")
	if err != nil {
		return "", err
	}
	tempDir, err := c.newTempDir()
	if err != nil {
		return "", err
	}
	keepTempDir := false
	defer func() {
		if !keepTempDir {
			c.removeTempDir(tempDir)
		}
	}()
	binaryPath := filepath.Join(tempDir, "update.tmp")
	if err := c.download(ctx, binaryURL, binaryPath, maxBinary); err != nil {
		return "", err
	}
	checksumsPath := filepath.Join(tempDir, "sha256.txt")
	if err := c.download(ctx, checksumURL, checksumsPath, 1<<20); err != nil {
		return "", err
	}
	expected, err := checksumForFile(checksumsPath, assetName)
	if err != nil {
		return "", err
	}
	if err := VerifyChecksum(binaryPath, expected); err != nil {
		return "", err
	}
	verifiedPath := filepath.Join(tempDir, "verified.tmp")
	if err := os.Rename(binaryPath, verifiedPath); err != nil {
		return "", fmt.Errorf("prepare verified update: %w", err)
	}
	keepTempDir = true
	return verifiedPath, nil
}

func (c *Client) Install(ctx context.Context, release *Release, currentPath string) error {
	verifiedPath, err := c.DownloadAndVerify(ctx, release)
	if err != nil {
		return err
	}
	defer c.removeTempDir(filepath.Dir(verifiedPath))
	resolvedPath, err := filepath.EvalSymlinks(currentPath)
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	return replaceBinary(resolvedPath, verifiedPath)
}

func (c *Client) download(ctx context.Context, source, destination string, limit int64) error {
	if err := c.validateDownloadURL(source); err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return fmt.Errorf("create binary download request: %w", err)
	}
	request.Header.Set("User-Agent", projectName+"-updater")
	response, err := c.httpClient().Do(request)
	if err != nil {
		return fmt.Errorf("download update asset: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("update asset returned HTTP %d", response.StatusCode)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create update file: %w", err)
	}
	_, copyErr := io.Copy(file, io.LimitReader(response.Body, limit+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write update asset: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close update asset: %w", closeErr)
	}
	info, err := os.Stat(destination)
	if err != nil {
		return fmt.Errorf("inspect update asset: %w", err)
	}
	if info.Size() > limit {
		return fmt.Errorf("update asset exceeds size limit")
	}
	return nil
}

func (c *Client) apiBase() (string, error) {
	if c.APIBase == "" {
		return "", errors.New("release API base URL is empty")
	}
	parsed, err := url.Parse(c.APIBase)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !(c.AllowHTTP && parsed.Scheme == "http")) {
		return "", errors.New("release API base URL must be an absolute HTTPS URL")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (c *Client) validateDownloadURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !(c.AllowHTTP && parsed.Scheme == "http")) || parsed.User != nil {
		return errors.New("release asset URL must be an absolute HTTPS URL")
	}
	return nil
}

func (c *Client) newTempDir() (string, error) {
	base := c.TempRoot
	if base == "" {
		base = filepath.Join(os.TempDir(), projectOrg)
	}
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", fmt.Errorf("create updater temporary directory: %w", err)
	}
	dir, err := os.MkdirTemp(base, projectName+"-")
	if err != nil {
		return "", fmt.Errorf("create updater temporary directory: %w", err)
	}
	return dir, nil
}

func (c *Client) removeTempDir(dir string) {
	base := c.TempRoot
	if base == "" {
		base = filepath.Join(os.TempDir(), projectOrg)
	}
	resolvedBase, err := filepath.Abs(base)
	if err != nil {
		return
	}
	resolved, err := filepath.Abs(dir)
	if err != nil {
		return
	}
	relative, err := filepath.Rel(resolvedBase, resolved)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || !strings.HasPrefix(filepath.Base(relative), projectName+"-") {
		return
	}
	info, err := os.Lstat(resolved)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return
	}
	_ = os.RemoveAll(resolved)
}

func releaseAssetURL(release *Release, name string) (string, error) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			if asset.BrowserDownloadURL == "" {
				return "", fmt.Errorf("release asset %q has no download URL", name)
			}
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("release is missing required asset %q", name)
}

func checksumForFile(path, assetName string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open checksums: %w", err)
	}
	defer file.Close()
	decoder := io.LimitReader(file, 1<<20)
	contents, err := io.ReadAll(decoder)
	if err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			if len(fields[0]) != sha256.Size*2 {
				return "", errors.New("release checksum has an invalid SHA-256 value")
			}
			if _, err := hex.DecodeString(fields[0]); err != nil {
				return "", errors.New("release checksum has an invalid SHA-256 value")
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("release checksums have no entry for %q", assetName)
}

func VerifyChecksum(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open downloaded update: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("calculate downloaded update checksum: %w", err)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("update checksum mismatch")
	}
	return nil
}

func BinaryName(goos, goarch string) string {
	name := projectName + "-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func matchesBranch(release Release, branch string) bool {
	if !release.Prerelease {
		return true
	}
	isBeta := strings.HasSuffix(release.TagName, "-beta")
	isDevel := release.TagName == "devel"
	switch branch {
	case "beta":
		return isBeta
	case "devel":
		return isBeta || isDevel
	default:
		return false
	}
}

func compareVersions(left, right string) int {
	left = strings.TrimPrefix(strings.TrimSpace(left), "v")
	right = strings.TrimPrefix(strings.TrimSpace(right), "v")
	if left == right {
		return 0
	}
	leftCore, leftPrerelease, leftHasPrerelease := strings.Cut(left, "-")
	rightCore, rightPrerelease, rightHasPrerelease := strings.Cut(right, "-")
	leftParts := strings.Split(leftCore, ".")
	rightParts := strings.Split(rightCore, ".")
	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		leftPart, rightPart := "0", "0"
		if index < len(leftParts) {
			leftPart = leftParts[index]
		}
		if index < len(rightParts) {
			rightPart = rightParts[index]
		}
		leftValue, leftErr := parseVersionPart(leftPart)
		rightValue, rightErr := parseVersionPart(rightPart)
		if leftErr == nil && rightErr == nil {
			if leftValue > rightValue {
				return 1
			}
			if leftValue < rightValue {
				return -1
			}
		} else if leftPart != rightPart {
			if leftPart > rightPart {
				return 1
			}
			return -1
		}
	}
	if leftHasPrerelease && !rightHasPrerelease {
		return -1
	}
	if !leftHasPrerelease && rightHasPrerelease {
		return 1
	}
	if leftPrerelease > rightPrerelease {
		return 1
	}
	if leftPrerelease < rightPrerelease {
		return -1
	}
	return 0
}

func parseVersionPart(value string) (int64, error) {
	var result int64
	if value == "" {
		return 0, errors.New("empty version part")
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, errors.New("invalid version part")
		}
		result = result*10 + int64(digit-'0')
	}
	return result, nil
}
