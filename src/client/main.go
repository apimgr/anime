package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

var (
	Version    = "dev"
	CommitID   = "unknown"
	BuildEpoch = "0"
)

const projectName = "anime"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer) (runErr error) {
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ContinueOnError)
	fs.SetOutput(errOut)
	serverURL := fs.String("server", "", "Server base URL (required unless configured)")
	configName := fs.String("config", "", "Configuration file or profile name")
	output := fs.String("output", "", "Output format: table, json, yaml, plain, or csv")
	fs.StringVar(output, "format", "", "Output format: table, json, yaml, plain, or csv")
	showVersion := fs.Bool("version", false, "Print version")
	fs.BoolVar(showVersion, "v", false, "Print version")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")
	debug := fs.Bool("debug", false, "Enable debug output")
	color := fs.String("color", "auto", "Color output: auto, yes, no")
	lang := fs.String("lang", "auto", "Language for output")
	shell := fs.String("shell", "", "Shell integration: completions or init")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *help {
		fmt.Fprintf(out, "Usage: %s [flags] <random|list|search> [terms]\n\n", filepath.Base(os.Args[0]))
		fs.SetOutput(out)
		fs.PrintDefaults()
		fmt.Fprintln(out, "\nCommands:\n  random       Fetch a random quote\n  list         List quotes\n  search TEXT  Search quote, anime, and character text")
		return nil
	}
	if *showVersion {
		fmt.Fprintf(out, "%s %s (%s) built %s\n", filepath.Base(os.Args[0]), Version, CommitID, clientBuildDate())
		return nil
	}
	if *shell != "" {
		return printClientShell(out, *shell, fs.Args())
	}
	settings, err := loadCLIConfigFile(*configName)
	if err != nil {
		return err
	}
	logFile, err := initClientLog(settings)
	if err != nil {
		return err
	}
	defer func() {
		if runErr != nil {
			log.New(logFile, "", log.LstdFlags|log.LUTC).Print("command failed")
		} else if settings.Logging.Level == "debug" || settings.Logging.Level == "info" {
			log.New(logFile, "", log.LstdFlags|log.LUTC).Print("command completed")
		}
		if closeErr := logFile.Close(); runErr == nil && closeErr != nil {
			runErr = fmt.Errorf("close CLI log: %w", closeErr)
		}
	}()
	flagSet := make(map[string]bool)
	fs.Visit(func(item *flag.Flag) { flagSet[item.Name] = true })
	if !flagSet["output"] && !flagSet["format"] {
		*output = os.Getenv("ANIME_OUTPUT_FORMAT")
		if *output == "" {
			*output = settings.Output.Format
		}
	}
	if *output == "" {
		*output = "table"
	}
	if !flagSet["color"] {
		*color = os.Getenv("ANIME_COLOR")
		if *color == "" {
			*color = settings.Output.Color
		}
	}
	if *color == "" {
		*color = "auto"
	}
	if value := os.Getenv("ANIME_SERVER_API_VERSION"); value != "" {
		settings.Server.APIVersion = value
	}
	if value := os.Getenv("ANIME_SERVER_TIMEOUT"); value != "" {
		settings.Server.Timeout = value
	}
	if value := os.Getenv("ANIME_SERVER_RETRY"); value != "" {
		retry, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("ANIME_SERVER_RETRY must be an integer")
		}
		settings.Server.Retry = retry
	}
	if value := os.Getenv("ANIME_SERVER_RETRY_DELAY"); value != "" {
		settings.Server.RetryDelay = value
	}
	if value := os.Getenv("ANIME_DEBUG"); value != "" {
		settings.Debug = value == "true" || value == "1"
	}
	if *output != "table" && *output != "json" && *output != "yaml" && *output != "plain" && *output != "csv" {
		return errors.New("--output must be table, json, yaml, plain, or csv")
	}
	if *color != "auto" && *color != "yes" && *color != "no" {
		return errors.New("--color must be auto, yes, or no")
	}
	if os.Getenv("NO_COLOR") != "" && *color == "auto" {
		*color = "no"
	}
	if *lang == "auto" {
		*lang = strings.SplitN(os.Getenv("LANG"), ".", 2)[0]
		if *lang == "" {
			*lang = "en"
		}
	}
	if *lang != "en" && !strings.HasPrefix(*lang, "en_") && !strings.HasPrefix(*lang, "en-") {
		*lang = "en"
	}
	if *serverURL == "" {
		*serverURL = os.Getenv("ANIME_SERVER_PRIMARY")
	}
	if *serverURL == "" {
		*serverURL = settings.Server.Primary
	}
	if *serverURL == "" {
		return errors.New("server URL is required; use --server, ANIME_SERVER_PRIMARY, or configure server.primary in cli.yml")
	}
	base, err := url.Parse(*serverURL)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" || base.User != nil || base.Fragment != "" {
		return errors.New("server URL must be an absolute http or https URL without credentials or fragment")
	}
	if settings.Server.Primary == "" && (*serverURL != "") {
		settings.Server.Primary = *serverURL
		if err := saveCLIConfigAt(settings, *configName); err != nil {
			return err
		}
	}
	if fs.NArg() == 0 && isTerminal() && settings.TUI.Enabled {
		return runTUI(*serverURL, settings.Server.APIVersion, settings.Server.Timeout, settings.TUI.Theme, settings.TUI.Mouse, settings.TUI.Unicode)
	}
	if fs.NArg() == 0 {
		return errors.New("command required: random, list, or search")
	}
	if !validAPIVersion(settings.Server.APIVersion) {
		return errors.New("server.api_version must use the form v followed by a version number")
	}
	endpoint := "/api/" + settings.Server.APIVersion + "/random"
	query := url.Values{}
	switch fs.Arg(0) {
	case "random":
	case "list":
		endpoint = "/api/" + settings.Server.APIVersion + "/quotes"
	case "search":
		if fs.NArg() < 2 {
			return errors.New("search requires a query")
		}
		endpoint = "/api/" + settings.Server.APIVersion + "/quotes"
		query.Set("q", strings.Join(fs.Args()[1:], " "))
	default:
		return fmt.Errorf("unknown command %q; use random, list, or search", fs.Arg(0))
	}
	base.Path = strings.TrimRight(base.Path, "/") + endpoint
	base.RawQuery = query.Encode()
	timeout, err := time.ParseDuration(settings.Server.Timeout)
	if err != nil || timeout <= 0 {
		return errors.New("server.timeout must be a positive duration")
	}
	if settings.Server.Retry < 0 {
		return errors.New("server.retry must not be negative")
	}
	retryDelay, err := time.ParseDuration(settings.Server.RetryDelay)
	if err != nil || retryDelay < 0 {
		return errors.New("server.retry_delay must be a non-negative duration")
	}
	requestURL := base.String()
	cacheDir := cliCacheDirectory()
	cacheTTL, err := time.ParseDuration(settings.Cache.TTL)
	if err != nil || cacheTTL < 0 {
		return errors.New("cache.ttl must be a non-negative duration")
	}
	cacheLimit, err := parseByteSize(settings.Cache.MaxSize)
	if err != nil {
		return fmt.Errorf("cache.max_size: %w", err)
	}
	var body []byte
	if settings.Cache.Enabled && fs.Arg(0) != "random" {
		body, err = readCachedResponse(cacheDir, requestURL, cacheTTL)
		if err != nil {
			return err
		}
	}
	if body == nil {
		if !settings.Output.Quiet && (*debug || settings.Debug || settings.Output.Verbose || os.Getenv("ANIME_DEBUG") == "true") {
			fmt.Fprintf(errOut, "GET %s\n", requestURL)
		}
		body, err = fetchResponse(requestURL, timeout, settings.Server.Retry, retryDelay)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		if settings.Cache.Enabled && fs.Arg(0) != "random" {
			if err := storeCachedResponse(cacheDir, requestURL, body, cacheLimit); err != nil {
				return err
			}
		}
	}
	if *output == "json" {
		_, err = fmt.Fprintln(out, strings.TrimSpace(string(body)))
		return err
	}
	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}
	useColor := *color == "yes" || (*color == "auto" && isTerminal())
	if os.Getenv("NO_COLOR") != "" {
		useColor = false
	}
	return renderFormat(out, value, *output, useColor)
}

func render(out io.Writer, value interface{}) error {
	writeQuote := func(item map[string]interface{}) {
		fmt.Fprintf(out, "Quote: %v\nCharacter: %v\nAnime: %v\n\n", item["quote"], item["character"], item["anime"])
	}
	if envelope, ok := value.(map[string]interface{}); ok {
		if data, exists := envelope["data"]; exists {
			value = data
		}
	}
	switch v := value.(type) {
	case map[string]interface{}:
		writeQuote(v)
	case []interface{}:
		for _, item := range v {
			if quote, ok := item.(map[string]interface{}); ok {
				writeQuote(quote)
			}
		}
	default:
		return errors.New("unexpected server response")
	}
	return nil
}

func renderFormat(out io.Writer, value interface{}, format string, useColor bool) error {
	if envelope, ok := value.(map[string]interface{}); ok {
		if data, exists := envelope["data"]; exists {
			value = data
		}
	}
	if format == "json" {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	}
	quotes, err := responseQuotes(value)
	if err != nil {
		return err
	}
	switch format {
	case "plain":
		for _, quote := range quotes {
			if _, err := fmt.Fprintln(out, quote.Quote); err != nil {
				return err
			}
		}
		return nil
	case "csv":
		writer := csv.NewWriter(out)
		if err := writer.Write([]string{"anime", "character", "quote"}); err != nil {
			return err
		}
		for _, quote := range quotes {
			if err := writer.Write([]string{quote.Anime, quote.Character, quote.Quote}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case "yaml":
		return yaml.NewEncoder(out).Encode(value)
	case "table":
		header := "ANIME\tCHARACTER\tQUOTE"
		if useColor {
			header = "\033[1m" + header + "\033[0m"
		}
		if _, err := fmt.Fprintln(out, header); err != nil {
			return err
		}
		for _, quote := range quotes {
			if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n", quote.Anime, quote.Character, quote.Quote); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func fetchResponse(requestURL string, timeout time.Duration, retries int, retryDelay time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		request, err := http.NewRequest(http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", projectName+"-cli/"+Version)
		response, err := client.Do(request)
		if err != nil {
			lastErr = err
		} else {
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
			closeErr := response.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if closeErr != nil {
				lastErr = closeErr
			} else if response.StatusCode >= 200 && response.StatusCode < 300 {
				return body, nil
			} else {
				lastErr = fmt.Errorf("server returned %s", response.Status)
				if response.StatusCode < 500 || response.StatusCode > 599 {
					return nil, lastErr
				}
			}
		}
		if attempt < retries && retryDelay > 0 {
			time.Sleep(retryDelay)
		}
	}
	return nil, lastErr
}

func validAPIVersion(version string) bool {
	if len(version) < 2 || version[0] != 'v' {
		return false
	}
	for _, digit := range version[1:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func cliCacheDirectory() string {
	_, dirs, err := cliDirectories()
	if err != nil || len(dirs) < 3 {
		return ""
	}
	return dirs[2]
}

func cachePath(cacheDir, requestURL string) string {
	hash := sha256.Sum256([]byte(requestURL))
	return filepath.Join(cacheDir, hex.EncodeToString(hash[:])+".cache")
}

func readCachedResponse(cacheDir, requestURL string, ttl time.Duration) ([]byte, error) {
	if cacheDir == "" || ttl == 0 {
		return nil, nil
	}
	path := cachePath(cacheDir, requestURL)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect cached response: %w", err)
	}
	if time.Since(info.ModTime()) > ttl {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove expired cached response: %w", err)
		}
		return nil, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cached response: %w", err)
	}
	return body, nil
}

func storeCachedResponse(cacheDir, requestURL string, body []byte, maxSize int64) error {
	if cacheDir == "" || maxSize == 0 || int64(len(body)) > maxSize {
		return nil
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("list response cache: %w", err)
	}
	type cacheEntry struct {
		path string
		size int64
		mod  time.Time
	}
	var cached []cacheEntry
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".cache" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect response cache: %w", err)
		}
		cached = append(cached, cacheEntry{path: filepath.Join(cacheDir, entry.Name()), size: info.Size(), mod: info.ModTime()})
		total += info.Size()
	}
	path := cachePath(cacheDir, requestURL)
	var previousSize int64
	for _, item := range cached {
		if item.path == path {
			previousSize = item.size
		}
	}
	needed := total - previousSize + int64(len(body)) - maxSize
	if needed > 0 {
		sort.Slice(cached, func(i, j int) bool { return cached[i].mod.Before(cached[j].mod) })
		for _, item := range cached {
			if item.path == path {
				continue
			}
			if err := os.Remove(item.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("evict response cache: %w", err)
			}
			needed -= item.size
			if needed <= 0 {
				break
			}
		}
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		return fmt.Errorf("write response cache: %w", err)
	}
	return nil
}

func parseByteSize(value string) (int64, error) {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return 0, errors.New("value is empty")
	}
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(value, "KB"):
		multiplier = 1 << 10
		value = strings.TrimSpace(strings.TrimSuffix(value, "KB"))
	case strings.HasSuffix(value, "MB"):
		multiplier = 1 << 20
		value = strings.TrimSpace(strings.TrimSuffix(value, "MB"))
	case strings.HasSuffix(value, "GB"):
		multiplier = 1 << 30
		value = strings.TrimSpace(strings.TrimSuffix(value, "GB"))
	case strings.HasSuffix(value, "B"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "B"))
	}
	amount, err := strconv.ParseInt(value, 10, 64)
	if err != nil || amount < 0 || amount > int64(^uint64(0)>>1)/multiplier {
		return 0, errors.New("value must be a non-negative byte size")
	}
	return amount * multiplier, nil
}

func responseQuotes(value interface{}) ([]quoteRecord, error) {
	decode := func(value interface{}) (quoteRecord, error) {
		encoded, err := json.Marshal(value)
		if err != nil {
			return quoteRecord{}, err
		}
		var quote quoteRecord
		if err := json.Unmarshal(encoded, &quote); err != nil {
			return quoteRecord{}, err
		}
		return quote, nil
	}
	switch items := value.(type) {
	case map[string]interface{}:
		quote, err := decode(items)
		return []quoteRecord{quote}, err
	case []interface{}:
		quotes := make([]quoteRecord, 0, len(items))
		for _, item := range items {
			quote, err := decode(item)
			if err != nil {
				return nil, err
			}
			quotes = append(quotes, quote)
		}
		return quotes, nil
	default:
		return nil, errors.New("unexpected server response")
	}
}

type cliSettings struct {
	Server struct {
		Primary    string `yaml:"primary"`
		APIVersion string `yaml:"api_version"`
		Timeout    string `yaml:"timeout"`
		Retry      int    `yaml:"retry"`
		RetryDelay string `yaml:"retry_delay"`
	} `yaml:"server"`
	Auth struct {
		Token     string `yaml:"token"`
		TokenFile string `yaml:"token_file"`
	} `yaml:"auth"`
	Output struct {
		Format  string `yaml:"format"`
		Color   string `yaml:"color"`
		Pager   string `yaml:"pager"`
		Quiet   bool   `yaml:"quiet"`
		Verbose bool   `yaml:"verbose"`
	} `yaml:"output"`
	TUI struct {
		Enabled bool   `yaml:"enabled"`
		Theme   string `yaml:"theme"`
		Mouse   bool   `yaml:"mouse"`
		Unicode bool   `yaml:"unicode"`
	} `yaml:"tui"`
	Logging struct {
		Level    string `yaml:"level"`
		File     string `yaml:"file"`
		MaxSize  string `yaml:"max_size"`
		MaxFiles int    `yaml:"max_files"`
	} `yaml:"logging"`
	Cache struct {
		Enabled bool   `yaml:"enabled"`
		TTL     string `yaml:"ttl"`
		MaxSize string `yaml:"max_size"`
	} `yaml:"cache"`
	Update struct {
		Auto          bool   `yaml:"auto"`
		CheckInterval string `yaml:"check_interval"`
		Channel       string `yaml:"channel"`
	} `yaml:"update"`
	Debug bool `yaml:"debug"`
}

func cliDirectories() (string, []string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", nil, err
	}
	if runtime.GOOS != "windows" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", nil, err
		}
		base = filepath.Join(home, ".config")
	}
	configDir := filepath.Join(base, "apimgr", "anime")
	if runtime.GOOS == "windows" {
		localBase := os.Getenv("LOCALAPPDATA")
		if localBase == "" {
			return "", nil, errors.New("LOCALAPPDATA is not set")
		}
		return filepath.Join(configDir, "cli.yml"), []string{configDir, filepath.Join(localBase, "apimgr", "anime", "data"), filepath.Join(localBase, "apimgr", "anime", "cache"), filepath.Join(localBase, "apimgr", "anime", "log")}, nil
	}
	dataBase, err := os.UserHomeDir()
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(configDir, "cli.yml"), []string{configDir, filepath.Join(dataBase, ".local", "share", "apimgr", "anime"), filepath.Join(dataBase, ".cache", "apimgr", "anime"), filepath.Join(dataBase, ".local", "log", "apimgr", "anime")}, nil
}

func loadCLIConfig() (*cliSettings, error) {
	return loadCLIConfigFile("")
}

func loadCLIConfigFile(configName string) (*cliSettings, error) {
	path, dirs, err := cliDirectories()
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create user directory: %w", err)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			return nil, fmt.Errorf("secure user directory: %w", err)
		}
	}
	if configName != "" {
		path, err = resolveCLIConfigPath(path, configName)
		if err != nil {
			return nil, err
		}
	}
	settings := defaultCLISettings()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if configName != "" {
			return nil, fmt.Errorf("configuration file %q does not exist", path)
		}
		return settings, saveCLIConfigAt(settings, "")
	}
	if err != nil {
		return nil, fmt.Errorf("read cli.yml: %w", err)
	}
	if err := yaml.Unmarshal(data, settings); err != nil {
		return nil, fmt.Errorf("parse cli.yml: %w", err)
	}
	return settings, nil
}

func defaultCLISettings() *cliSettings {
	settings := &cliSettings{}
	settings.Server.APIVersion = "v1"
	settings.Server.Timeout = "30s"
	settings.Server.Retry = 3
	settings.Server.RetryDelay = "1s"
	settings.Output.Format = "table"
	settings.Output.Color = "auto"
	settings.Output.Pager = "auto"
	settings.TUI.Enabled = true
	settings.TUI.Theme = "dark"
	settings.TUI.Mouse = true
	settings.TUI.Unicode = true
	settings.Logging.Level = "warn"
	settings.Logging.MaxSize = "10MB"
	settings.Logging.MaxFiles = 5
	settings.Cache.Enabled = true
	settings.Cache.TTL = "5m"
	settings.Cache.MaxSize = "100MB"
	settings.Update.CheckInterval = "per_invocation"
	settings.Update.Channel = "stable"
	return settings
}

func initClientLog(settings *cliSettings) (*os.File, error) {
	level := strings.ToLower(settings.Logging.Level)
	if level != "debug" && level != "info" && level != "warn" && level != "error" {
		return nil, errors.New("logging.level must be debug, info, warn, or error")
	}
	if settings.Logging.MaxFiles < 0 {
		return nil, errors.New("logging.max_files must not be negative")
	}
	maxSize, err := parseByteSize(settings.Logging.MaxSize)
	if err != nil || maxSize == 0 {
		return nil, errors.New("logging.max_size must be a positive byte size")
	}
	path := settings.Logging.File
	if path == "" {
		_, dirs, err := cliDirectories()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dirs[3], "cli.log")
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create CLI log directory: %w", err)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			return nil, fmt.Errorf("secure CLI log directory: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("inspect CLI log directory: %w", err)
	}
	if info, err := os.Stat(path); err == nil && info.Size() >= maxSize {
		if err := rotateClientLog(path, settings.Logging.MaxFiles); err != nil {
			return nil, err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect CLI log: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open CLI log: %w", err)
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure CLI log: %w", err)
	}
	return file, nil
}

func rotateClientLog(logPath string, maxFiles int) error {
	if maxFiles == 0 {
		if err := os.Truncate(logPath, 0); err != nil {
			return fmt.Errorf("truncate CLI log: %w", err)
		}
		return nil
	}
	if err := os.Remove(logPath + "." + strconv.Itoa(maxFiles)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove old CLI log: %w", err)
	}
	for index := maxFiles - 1; index >= 1; index-- {
		oldPath := logPath + "." + strconv.Itoa(index)
		newPath := logPath + "." + strconv.Itoa(index+1)
		if err := os.Rename(oldPath, newPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("rotate CLI log: %w", err)
		}
	}
	if err := os.Rename(logPath, logPath+".1"); err != nil {
		return fmt.Errorf("rotate CLI log: %w", err)
	}
	return nil
}

func resolveCLIConfigPath(defaultPath, name string) (string, error) {
	if strings.HasPrefix(name, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		name = filepath.Join(home, name[2:])
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(filepath.Dir(defaultPath), name)
	}
	if ext := filepath.Ext(name); ext == ".yml" || ext == ".yaml" {
		return name, nil
	}
	if filepath.Ext(name) != "" {
		return "", errors.New("configuration profile must use .yml or .yaml")
	}
	if _, err := os.Stat(name + ".yml"); err == nil {
		return name + ".yml", nil
	}
	if _, err := os.Stat(name + ".yaml"); err == nil {
		return name + ".yaml", nil
	}
	return name + ".yml", nil
}

func saveCLIConfig(settings *cliSettings) error {
	return saveCLIConfigAt(settings, "")
}

func saveCLIConfigAt(settings *cliSettings, configName string) error {
	path, _, err := cliDirectories()
	if err != nil {
		return err
	}
	if configName != "" {
		path, err = resolveCLIConfigPath(path, configName)
		if err != nil {
			return err
		}
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write cli.yml: %w", err)
	}
	return os.Chmod(path, 0600)
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func clientBuildDate() string {
	seconds, err := strconv.ParseInt(BuildEpoch, 10, 64)
	if err != nil || seconds <= 0 {
		return "N/A"
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func printClientShell(out io.Writer, command string, args []string) error {
	shell := ""
	if len(args) > 0 {
		shell = args[0]
	} else {
		shell = filepath.Base(os.Getenv("SHELL"))
	}
	if shell == "" {
		shell = "bash"
	}
	name := filepath.Base(os.Args[0])
	if command == "init" {
		switch shell {
		case "bash", "zsh":
			fmt.Fprintf(out, "source <(%s --shell completions %s)\n", name, shell)
		case "fish":
			fmt.Fprintf(out, "%s --shell completions fish | source\n", name)
		case "sh", "dash", "ksh":
			fmt.Fprintf(out, "eval \"$(%s --shell completions %s)\"\n", name, shell)
		case "powershell", "pwsh":
			fmt.Fprintf(out, "Invoke-Expression (& %s --shell completions powershell)\n", name)
		default:
			return fmt.Errorf("unsupported shell %q", shell)
		}
		return nil
	}
	if command != "completions" {
		return errors.New("--shell expects completions or init")
	}
	commands := "random list search"
	flags := "--server --config --output --format --color --lang --shell --debug --help --version"
	switch shell {
	case "bash":
		fmt.Fprintf(out, "_%s_complete() {\n  local cur prev\n  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n  prev=\"${COMP_WORDS[COMP_CWORD-1]}\"\n  case \"$prev\" in --server|--config|--output|--format|--color|--lang|--shell) return ;; esac\n  COMPREPLY=( $(compgen -W '%s %s' -- \"$cur\") )\n}\ncomplete -F _%s_complete %s\n", completionFunctionName(name), commands, flags, completionFunctionName(name), name)
	case "zsh":
		fmt.Fprintf(out, "#compdef %s\n_arguments '1:command:(%s)' '--server[Server base URL]:URL:' '--config[Configuration profile or file]:file:_files' '--output[Output format]:(table json yaml plain csv)' '--format[Output format]:(table json yaml plain csv)' '--color[Color output]:(auto yes no)' '--lang[Language code]:' '--shell[Shell integration]:(completions init)' '--debug[Enable debug output]' '--help[Show help]' '--version[Show version]'\n", name, commands)
	case "fish":
		for _, item := range strings.Fields(commands) {
			fmt.Fprintf(out, "complete -c %s -f -n '__fish_use_subcommand' -a %s\n", name, item)
		}
		fmt.Fprintf(out, "complete -c %s -l server -r -d 'Server base URL'\ncomplete -c %s -l config -r -F -d 'Configuration profile or file'\ncomplete -c %s -l output -r -a 'table json yaml plain csv'\ncomplete -c %s -l format -r -a 'table json yaml plain csv'\ncomplete -c %s -l color -r -a 'auto yes no'\ncomplete -c %s -l lang -r\ncomplete -c %s -l shell -r -a 'completions init'\ncomplete -c %s -l debug\ncomplete -c %s -l help\ncomplete -c %s -l version\n", name, name, name, name, name, name, name, name, name, name)
	case "sh", "dash", "ksh":
		fmt.Fprintf(out, "_%s_candidates() { printf '%%s\\n' '%s %s'; }\n", completionFunctionName(name), commands, flags)
	case "powershell", "pwsh":
		fmt.Fprintf(out, "Register-ArgumentCompleter -Native -CommandName '%s' -ScriptBlock { param($wordToComplete, $commandAst, $cursorPosition) '%s %s'.Split(' ') | Where-Object { $_.StartsWith($wordToComplete) } | ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) } }\n", name, commands, flags)
	default:
		return fmt.Errorf("unsupported shell %q", shell)
	}
	return nil
}

func completionFunctionName(binary string) string {
	var builder strings.Builder
	for _, current := range strings.ToLower(binary) {
		if current >= 'a' && current <= 'z' || current >= '0' && current <= '9' {
			builder.WriteRune(current)
		} else {
			builder.WriteByte('_')
		}
	}
	return builder.String()
}
