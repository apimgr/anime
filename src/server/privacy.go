package server

import (
	"net/http"
	"strings"

	"github.com/apimgr/anime/src/common/i18n"
	"github.com/apimgr/anime/src/config"
)

func privacyAPIData(r *http.Request, s *Server) map[string]interface{} {
	ccpaCookie, ccpaErr := r.Cookie("ccpa_opt_out")
	privacy := s.cfg.Server.Privacy
	sold := privacy.Data.Sold
	sharing := make([]map[string]string, 0, len(privacy.Data.Sharing))
	for _, entry := range privacy.Data.Sharing {
		sharing = append(sharing, map[string]string{"condition": entry.Condition, "when": entry.When, "data": entry.Data})
	}
	services := make([]map[string]string, 0, len(privacy.ThirdParty.Services))
	for _, service := range privacy.ThirdParty.Services {
		services = append(services, map[string]string{"name": service.Name, "purpose": service.Purpose, "data_sent": service.DataSent, "policy_url": service.PolicyURL})
	}
	dataUsage := "Request data supports quote search, service operation, and abuse prevention. The application does not sell visitor data or configure analytics tracking."
	consentMessage := privacy.Consent.Message
	if sold {
		dataUsage = "Request data supports quote search, service operation, and abuse prevention. Visitor data may be shared with or sold to third parties as described by the service operator."
		consentMessage = privacy.Consent.MessageIfSold
	}
	if consentMessage == "" {
		key := "consent.message"
		if sold {
			key = "consent.message_sold"
		}
		consentMessage = i18n.Translate(i18n.LangFromRequest(r), key)
	}
	return map[string]interface{}{
		"summary": map[string]interface{}{
			"data_stored_on_server": privacy.Data.StoredOnServer,
			"data_sold":             sold,
			"user_control":          true,
		},
		"cookies": map[string]interface{}{
			"essential":   map[string]interface{}{"enabled": true, "description": privacy.Cookies.Essential.Description},
			"preferences": map[string]interface{}{"enabled": privacy.Cookies.Preferences.Enabled, "description": privacy.Cookies.Preferences.Description},
			"analytics":   map[string]interface{}{"enabled": false, "description": analyticsCookieDescription(privacy.Cookies.Analytics, sold)},
		},
		"data": map[string]interface{}{
			"sold": sold, "stored_on_server": privacy.Data.StoredOnServer,
			"sharing": sharing,
		},
		"tracking": map[string]interface{}{"enabled": false, "type": "", "type_name": ""},
		"retention": map[string]interface{}{
			"period":           privacy.Retention.Period,
			"export_available": privacy.Retention.ExportAvailable, "deletion_available": privacy.Retention.DeletionAvailable,
		},
		"third_party": map[string]interface{}{"services": services},
		"ccpa":        map[string]interface{}{"applicable": sold, "opt_out_url": "/server/privacy#ccpa-opt-out", "user_opted_out": ccpaErr == nil && ccpaCookie.Value == "true"},
		"content": map[string]string{
			"consent_message":  consentMessage,
			"data_usage":       dataUsage,
			"operator_content": string(s.operatorHTML(r, "privacy")),
		},
	}
}

func analyticsCookieDescription(cookie config.PrivacyAnalyticsCookie, sold bool) string {
	description := strings.TrimSpace(cookie.Description)
	suffix := cookie.DescriptionSuffixNotSold
	if sold {
		suffix = cookie.DescriptionSuffixSold
	}
	if strings.TrimSpace(suffix) != "" {
		if description != "" {
			description += " "
		}
		description += strings.TrimSpace(suffix)
	}
	return description
}
