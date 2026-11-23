package server

import (
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type disclosurePolicySection struct {
	Title   string
	Content string
	Link    string
}

func disclosurePolicySections(host string) []disclosurePolicySection {
	if host == "" {
		host = "this service"
	}
	return []disclosurePolicySection{
		{Title: "Scope", Content: "The application served at " + host + " and its API are in scope. Other hosts in the DOMAIN list, the Tor address, and the I2P address are also in scope when published."},
		{Title: "Out of scope", Content: "Denial of service and volumetric testing, social engineering of operators or users, physical attacks, findings in third-party services or dependencies without an exploitable path here, automated scanner output without a demonstrated impact, and missing best-practice headers on non-sensitive pages are out of scope."},
		{Title: "Coordinated disclosure", Content: "Please report privately through /server/security and allow 90 days from the report to publish. The window ends early once a fix is released and may be extended by agreement."},
		{Title: "Safe harbor", Content: "Good-faith research that follows this policy, avoids privacy violations and service disruption, accesses only what is needed to show the issue, and stops when it has done so is authorized. No legal action will be taken for it."},
		{Title: "Reporting", Content: "Report a vulnerability through", Link: "/server/security"},
	}
}

func disclosurePolicyMarkdown(host string) string {
	sections := disclosurePolicySections(host)
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		content := section.Content
		if section.Link != "" {
			content += " [" + section.Link + "](" + section.Link + ")"
		}
		parts = append(parts, "## "+section.Title+"\n\n"+content)
	}
	return strings.Join(parts, "\n\n")
}

func requestHostname(r *http.Request) string {
	host := r.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		return name
	}
	return strings.Trim(host, "[]")
}

func (s *Server) handleSecurityPage(w http.ResponseWriter, r *http.Request) {
	page := map[string]string{
		"/server/security":        "security",
		"/server/security/policy": "security-policy",
		"/server/security/thanks": "security-thanks",
	}[r.URL.Path]
	if page == "" {
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	titles := map[string]string{
		"security":        "Security",
		"security-policy": "Disclosure policy",
		"security-thanks": "Security acknowledgments",
	}
	data := map[string]interface{}{
		"Title": titles[page], "Description": "Security reporting and disclosure information for Anime Quotes.",
		"Page": page, "ContentTemplate": "security-content", "Version": Version,
		"SecurityExpires": time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339),
		"ReportURL":       s.securityReportURL(),
	}
	if page == "security" && s.cfg.Server.Pages.Contact.Enabled {
		data["SecurityReportURL"] = requestOrigin(r) + "/server/contact?security_id=" + s.securityID(time.Now())
		data["OtherReportURL"] = requestOrigin(r) + "/server/contact"
	}
	if page == "security" {
		data["SecurityEmail"] = s.securityContactEmail(r)
		if _, err := s.securityPublicKey(); err == nil {
			data["PGPKeyURL"] = requestOrigin(r) + "/.well-known/pgp-key.asc"
		}
	}
	if page == "security-policy" {
		data["DisclosurePolicy"] = disclosurePolicySections(requestHostname(r))
	}
	if page == "security-thanks" && s.database != nil && s.database.SQL != nil {
		if entries, err := s.listSecurityThanks(r.Context()); err == nil {
			data["ThanksEntries"] = entries
		}
	}
	if err := s.renderPage(w, r, data); err != nil {
		log.Printf("Error rendering security page: %v", err)
	}
}

func (s *Server) handleSecurityJSON(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	origin := requestOrigin(r)
	var data interface{}
	switch path {
	case "/server/security":
		contacts := []map[string]string{{"type": "github", "value": s.securityReportURL()}}
		if s.cfg.Server.Pages.Contact.Enabled {
			contacts = append(contacts, map[string]string{"type": "contact_form", "value": origin + "/server/contact?security_id=" + s.securityID(time.Now())})
		}
		securityEmail := s.securityContactEmail(r)
		if securityEmail != "" {
			contacts = append(contacts, map[string]string{"type": "email", "value": "mailto:" + securityEmail})
		}
		pgpAvailable := false
		fingerprint := ""
		if _, err := s.securityPublicKey(); err == nil {
			fingerprint = s.securityKeyFingerprint()
			pgpAvailable = fingerprint != ""
		}
		var pgpURL interface{}
		var pgpFingerprint interface{}
		if pgpAvailable {
			pgpURL = origin + "/.well-known/pgp-key.asc"
			pgpFingerprint = fingerprint
		}
		data = map[string]interface{}{
			"contacts":   contacts,
			"expires":    time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339),
			"pgp":        map[string]interface{}{"available": pgpAvailable, "url": pgpURL, "fingerprint": pgpFingerprint},
			"policy_url": origin + "/server/security/policy",
			"thanks_url": origin + "/server/security/thanks",
		}
		if s.cfg.Server.Pages.Contact.Enabled {
			data.(map[string]interface{})["report_url"] = origin + "/server/contact"
		}
	case "/server/security/policy":
		data = map[string]string{"policy": disclosurePolicyMarkdown(requestHostname(r))}
	case "/server/security/thanks":
		entries := []map[string]string{}
		if s.database != nil && s.database.SQL != nil {
			entries, _ = s.listSecurityThanks(r.Context())
		}
		data = map[string]interface{}{"entries": entries}
	default:
		s.renderStatusError(w, r, http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, data)
}

func (s *Server) securityContactEmail(r *http.Request) string {
	if value := strings.TrimSpace(s.cfg.WebSecurity.Contact); value != "" {
		return value
	}
	if value := strings.TrimSpace(s.cfg.Server.Contact.Security.Email); value != "" {
		return value
	}
	if value := strings.TrimSpace(s.cfg.Server.Contact.Admin.Email); value != "" {
		return value
	}
	if host := normalizeContactHost(resolvedRequestHost(r)); host != "" {
		return "security@" + host
	}
	return ""
}

func (s *Server) securityReportURL() string {
	if value := strings.TrimSpace(s.cfg.WebSecurity.ReportURL); value != "" {
		return value
	}
	return "https://github.com/apimgr/anime/security/advisories/new"
}
