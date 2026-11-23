package server

import (
	"sort"

	"github.com/apimgr/anime/src/common/i18n"
)

type serverPageLink struct {
	Path        string
	TitleKey    string
	DescKey     string
	Title       string
	Description string
	Order       int
	Hub         string
	Sitemap     bool
}

var serverPageLinks = []serverPageLink{
	{
		Path:     "/server/preferences/tokens",
		TitleKey: "preferences.tokens.title",
		DescKey:  "preferences.tokens.description",
		Order:    5,
		Hub:      "preferences",
	},
	{
		Path:     "/server/help/mcp",
		TitleKey: "help.mcp.title",
		DescKey:  "help.mcp.description",
		Order:    10,
		Hub:      "help",
		Sitemap:  true,
	},
	{
		Path:     "/server/preferences/sync",
		TitleKey: "preferences.sync.title",
		DescKey:  "preferences.sync.description",
		Order:    10,
		Hub:      "preferences",
	},
}

func registeredSitemapPages() []serverPageLink {
	pages := make([]serverPageLink, 0)
	for _, page := range serverPageLinks {
		if page.Sitemap {
			pages = append(pages, page)
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Order < pages[j].Order })
	return pages
}

func registeredServerPages(hub string, language ...string) []serverPageLink {
	lang := "en"
	if len(language) > 0 {
		lang = language[0]
	}
	pages := make([]serverPageLink, 0)
	for _, page := range serverPageLinks {
		if page.Hub == hub {
			page.Title = i18n.Translate(lang, page.TitleKey)
			page.Description = i18n.Translate(lang, page.DescKey)
			pages = append(pages, page)
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Order < pages[j].Order })
	return pages
}

func helpPageAPIEntries(language ...string) []map[string]string {
	pages := registeredServerPages("help", language...)
	entries := make([]map[string]string, 0, len(pages))
	for _, page := range pages {
		entries = append(entries, map[string]string{"title": page.Title, "path": page.Path})
	}
	return entries
}
