package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiModel struct {
	serverURL  string
	apiVersion string
	timeout    string
	theme      string
	mouse      bool
	unicode    bool
	width      int
	height     int
	query      string
	searching  bool
	loading    bool
	message    string
	quotes     []quoteRecord
}

type quoteRecord struct {
	Anime     string `json:"anime"`
	Character string `json:"character"`
	Quote     string `json:"quote"`
}

type apiEnvelope struct {
	Data json.RawMessage `json:"data"`
}

type quotesLoaded struct {
	quotes []quoteRecord
	err    error
}

func runTUI(serverURL, apiVersion, timeout, theme string, mouse, unicode bool) error {
	options := []tea.ProgramOption{tea.WithAltScreen()}
	if mouse {
		options = append(options, tea.WithMouseCellMotion())
	}
	_, err := tea.NewProgram(tuiModel{serverURL: serverURL, apiVersion: apiVersion, timeout: timeout, theme: theme, mouse: mouse, unicode: unicode, width: 80, height: 24}, options...).Run()
	return err
}

func (m tuiModel) Init() tea.Cmd { return m.load("random", "") }

func (m tuiModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case quotesLoaded:
		m.loading = false
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		m.quotes, m.message = msg.quotes, fmt.Sprintf("%d quotes", len(msg.quotes))
	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
			case "enter":
				m.searching = false
				m.loading = true
				return m, m.load("search", m.query)
			case "backspace", "ctrl+h":
				if len(m.query) > 0 {
					m.query = m.query[:len(m.query)-1]
				}
			default:
				if len(msg.String()) == 1 {
					m.query += msg.String()
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, m.load("random", "")
		case "l":
			m.loading = true
			return m, m.load("list", "")
		case "/":
			m.searching = true
			m.query = ""
		case "?":
			m.message = "r random · l list · / search · ? help · q quit"
		}
	}
	return m, nil
}

func (m tuiModel) View() string {
	width := m.width
	if width < 20 {
		width = 20
	}
	if width > 120 {
		width = 120
	}
	title := "Anime Quotes"
	if os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		style := lipgloss.NewStyle().Bold(true)
		switch m.theme {
		case "light":
			style = style.Foreground(lipgloss.Color("#5b21b6"))
		case "system":
			style = style.Foreground(lipgloss.AdaptiveColor{Light: "#5b21b6", Dark: "#bd93f9"})
		default:
			style = style.Foreground(lipgloss.Color("#bd93f9"))
		}
		title = style.Render(title)
	}
	body := []string{title, "Server: " + m.serverURL, ""}
	if m.searching {
		cursor, separator := "▏", " · "
		if !m.unicode {
			cursor, separator = "_", " | "
		}
		body = append(body, "Search: "+m.query+cursor, "Press Enter to search"+separator+"Esc to cancel")
	}
	if m.loading {
		loading := "Loading quotes…"
		if !m.unicode {
			loading = "Loading quotes..."
		}
		body = append(body, loading)
	} else {
		visible := m.quotes
		maxItems := m.height - 7
		if maxItems < 1 {
			maxItems = 1
		}
		if len(visible) > maxItems {
			visible = visible[:maxItems]
		}
		for _, quote := range visible {
			line := fmt.Sprintf("%s\n— %s · %s", quote.Quote, quote.Character, quote.Anime)
			border := lipgloss.RoundedBorder()
			if !m.unicode {
				border = lipgloss.NormalBorder()
			}
			body = append(body, lipgloss.NewStyle().Width(width-4).Border(border).Padding(0, 1).Render(line), "")
		}
		if len(m.quotes) > len(visible) {
			body = append(body, fmt.Sprintf("Showing %d of %d. Use search to narrow the collection.", len(visible), len(m.quotes)))
		}
	}
	body = append(body, m.message, "r random · l list · / search · ? help · q quit")
	lines := strings.Join(body, "\n")
	if m.height > 0 {
		return lipgloss.NewStyle().Width(width).MaxHeight(m.height).Render(lines)
	}
	return lines
}

func (m tuiModel) load(action, query string) tea.Cmd {
	return func() tea.Msg {
		version := m.apiVersion
		if version == "" {
			version = "v1"
		}
		endpoint := "/api/" + version + "/random"
		params := url.Values{}
		if action != "random" {
			endpoint = "/api/" + version + "/quotes"
		}
		if action == "search" {
			params.Set("q", query)
		}
		base, err := url.Parse(m.serverURL)
		if err != nil {
			return quotesLoaded{err: err}
		}
		base.Path = strings.TrimRight(base.Path, "/") + endpoint
		base.RawQuery = params.Encode()
		request, err := http.NewRequest(http.MethodGet, base.String(), nil)
		if err != nil {
			return quotesLoaded{err: err}
		}
		request.Header.Set("User-Agent", projectName+"-cli/"+Version)
		timeout, err := time.ParseDuration(m.timeout)
		if err != nil || timeout <= 0 {
			timeout = 30 * time.Second
		}
		response, err := (&http.Client{Timeout: timeout}).Do(request)
		if err != nil {
			return quotesLoaded{err: err}
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return quotesLoaded{err: fmt.Errorf("server returned %s", response.Status)}
		}
		var envelope apiEnvelope
		if err := json.NewDecoder(io.LimitReader(response.Body, 10<<20)).Decode(&envelope); err != nil {
			return quotesLoaded{err: err}
		}
		if action == "random" {
			var quote quoteRecord
			if err := json.Unmarshal(envelope.Data, &quote); err != nil {
				return quotesLoaded{err: err}
			}
			return quotesLoaded{quotes: []quoteRecord{quote}}
		}
		var quotes []quoteRecord
		if err := json.Unmarshal(envelope.Data, &quotes); err != nil {
			return quotesLoaded{err: err}
		}
		return quotesLoaded{quotes: quotes}
	}
}
