package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/anime/src/config"
	"github.com/apimgr/anime/src/database"
)

func MaintainSecurityReports(ctx context.Context, db *database.Database, cfg *config.Config, args []string, output io.Writer) error {
	if db == nil || db.SQL == nil || len(args) == 0 {
		return fmt.Errorf("usage: anime --maintenance security {list [--status STATUS]|show TRACKING_ID|update TRACKING_ID [options]}")
	}
	switch args[0] {
	case "list":
		status := ""
		if len(args) == 3 && args[1] == "--status" {
			status = args[2]
		} else if len(args) != 1 {
			return fmt.Errorf("usage: anime --maintenance security list [--status STATUS]")
		}
		query := `SELECT tracking_id, status, severity, component, created_at, updated_at FROM security_reports`
		values := []interface{}{}
		if status != "" {
			if !validSecurityReportStatus(status) {
				return fmt.Errorf("invalid report status %q", status)
			}
			query += ` WHERE status = ?`
			values = append(values, status)
		}
		query += ` ORDER BY created_at DESC, tracking_id`
		rows, err := db.SQL.QueryContext(ctx, query, values...)
		if err != nil {
			return err
		}
		defer closeAndLog(rows, "security report rows")
		for rows.Next() {
			var id, reportStatus, severity, component, created, updated string
			if err := rows.Scan(&id, &reportStatus, &severity, &component, &created, &updated); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(output, "%s\t%s\t%s\t%s\t%s\t%s\n", id, reportStatus, severity, component, created, updated); err != nil {
				return err
			}
		}
		return rows.Err()
	case "show":
		if len(args) != 2 {
			return fmt.Errorf("usage: anime --maintenance security show TRACKING_ID")
		}
		var id, status, severity, component, comment, disclosure string
		var created, updated string
		err := db.SQL.QueryRowContext(ctx, `SELECT tracking_id, status, severity, component, researcher_comment, COALESCE(disclosure_date, ''), created_at, updated_at FROM security_reports WHERE tracking_id = ?`, args[1]).Scan(&id, &status, &severity, &component, &comment, &disclosure, &created, &updated)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "tracking_id: %s\nstatus: %s\nseverity: %s\ncomponent: %s\ncreated_at: %s\nupdated_at: %s\nresearcher_comment: %s\ndisclosure_date: %s\n", id, status, severity, component, created, updated, comment, disclosure)
		return err
	case "update":
		return updateSecurityReport(ctx, db, cfg, args[1:], output)
	default:
		return fmt.Errorf("unknown security report action %q", args[0])
	}
}

func updateSecurityReport(ctx context.Context, db *database.Database, cfg *config.Config, args []string, output io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: anime --maintenance security update TRACKING_ID [--status STATUS] [--comment TEXT] [--disclosure-date YYYY-MM-DD] [--severity LEVEL]")
	}
	id := args[0]
	updates := map[string]string{}
	for index := 1; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return fmt.Errorf("option %s requires a value", args[index])
		}
		key := strings.TrimPrefix(args[index], "--")
		if key != "status" && key != "comment" && key != "disclosure-date" && key != "severity" {
			return fmt.Errorf("unknown security report option %q", args[index])
		}
		updates[key] = args[index+1]
	}
	if len(updates) == 0 {
		return fmt.Errorf("at least one update field is required")
	}
	if value, exists := updates["status"]; exists && !validSecurityReportStatus(value) {
		return fmt.Errorf("invalid report status %q", value)
	}
	if value, exists := updates["severity"]; exists && !oneOf(value, "critical", "high", "medium", "low", "informational") {
		return fmt.Errorf("invalid report severity %q", value)
	}
	if value, exists := updates["comment"]; exists && (len([]rune(value)) > 2000 || strings.ContainsAny(value, "\x00\r")) {
		return fmt.Errorf("researcher comment must be plain text no longer than 2000 characters")
	}
	if value, exists := updates["disclosure-date"]; exists && value != "" {
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return fmt.Errorf("disclosure date must use YYYY-MM-DD")
		}
	}
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("security report transaction rollback failed: %v", err)
		}
	}()
	var previousStatus, severity, creditPreference, creditName string
	var closedAt sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT status, severity, credit_pref, COALESCE(credit_name, ''), closed_at FROM security_reports WHERE tracking_id = ?`, id).Scan(&previousStatus, &severity, &creditPreference, &creditName, &closedAt); err != nil {
		return err
	}
	nextStatus := updates["status"]
	if nextStatus == "" {
		nextStatus = previousStatus
	} else if !validStatusAdvance(previousStatus, nextStatus) {
		return fmt.Errorf("report status cannot move from %s to %s", previousStatus, nextStatus)
	}
	if value, exists := updates["severity"]; exists {
		severity = value
	}
	now := time.Now().UTC().Format(time.RFC3339)
	comment := updates["comment"]
	disclosure := updates["disclosure-date"]
	_, err = tx.ExecContext(ctx, `UPDATE security_reports SET status = ?, severity = ?, researcher_comment = CASE WHEN ? THEN ? ELSE researcher_comment END, disclosure_date = CASE WHEN ? THEN NULLIF(?, '') ELSE disclosure_date END, updated_at = ?, closed_at = CASE WHEN ? IN ('disclosed', 'wont_fix') THEN COALESCE(closed_at, ?) ELSE closed_at END WHERE tracking_id = ?`, nextStatus, severity, hasOption(updates, "comment"), comment, hasOption(updates, "disclosure-date"), disclosure, now, nextStatus, now, id)
	if err != nil {
		return err
	}
	if nextStatus == "disclosed" && previousStatus != "disclosed" && creditPreference != "no" {
		credit := creditName
		if creditPreference == "anon" {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_thanks WHERE credit LIKE 'Anonymous Researcher #%';`).Scan(&count); err != nil {
				return err
			}
			credit = fmt.Sprintf("Anonymous Researcher #%d", count+1)
		}
		if credit != "" {
			_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO security_thanks (tracking_id, credit, severity, disclosed_on) VALUES (?, ?, ?, ?)`, id, credit, severity, time.Now().UTC().Format("2006-01-02"))
			if err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if previousStatus != nextStatus {
		log.Printf("security.report_updated tracking_id=%s old_status=%s new_status=%s", id, previousStatus, nextStatus)
		if _, err := fmt.Fprintf(output, "security.report_updated tracking_id=%s old_status=%s new_status=%s\n", id, previousStatus, nextStatus); err != nil {
			return err
		}
	}
	if cfg != nil && hasOption(updates, "comment") || cfg != nil && hasOption(updates, "status") {
		sendSecurityStatusNotice(ctx, db, cfg, id, nextStatus, updates["comment"])
	}
	return nil
}

func sendSecurityStatusNotice(ctx context.Context, db *database.Database, cfg *config.Config, id, status, comment string) {
	var encrypted []byte
	if err := db.SQL.QueryRowContext(ctx, `SELECT contact_enc FROM security_reports WHERE tracking_id = ?`, id).Scan(&encrypted); err != nil || len(encrypted) == 0 {
		return
	}
	var contact map[string]string
	server := &Server{cfg: cfg}
	clear, err := server.decryptReportValue(encrypted)
	if err != nil || json.Unmarshal(clear, &contact) != nil || contact["email"] == "" {
		return
	}
	body := fmt.Sprintf("The status of your security report %s changed to %s.\n", id, status)
	if comment != "" {
		body += "\n" + comment + "\n"
	}
	if secret, secretErr := database.AppSecret(ctx, db.SQL, "installation_secret"); secretErr == nil {
		server.installationSecret = secret
		if signature, signErr := server.signSecurityNotice([]byte(body)); signErr == nil {
			body += "\nOpenPGP signature:\n" + string(signature)
		}
	}
	_ = server.sendSMTPMessage(strings.TrimSpace(cfg.Server.Notifications.Email.SMTP.Host), cfg.Server.Notifications.Email.SMTP.Port, cfg.Server.Notifications.Email, contact["email"], "localhost", "", "", "Security report status updated", body)
}

func validSecurityReportStatus(value string) bool {
	return oneOf(value, "received", "triaged", "confirmed", "patching", "disclosed", "wont_fix")
}

func validStatusAdvance(previous, next string) bool {
	if previous == next {
		return true
	}
	if previous == "disclosed" || previous == "wont_fix" {
		return false
	}
	if next == "wont_fix" {
		return true
	}
	order := map[string]int{"received": 0, "triaged": 1, "confirmed": 2, "patching": 3, "disclosed": 4}
	return order[previous] < order[next]
}

func hasOption(values map[string]string, key string) bool {
	_, exists := values[key]
	return exists
}

func CleanupSecurityReports(ctx context.Context, db *database.Database, retentionDays int, configDir string, now time.Time) error {
	if db == nil || db.SQL == nil {
		return fmt.Errorf("security report database is unavailable")
	}
	if retentionDays <= 0 {
		retentionDays = 365
	}
	cutoff := now.UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	if _, err := db.SQL.ExecContext(ctx, `DELETE FROM security_reports WHERE closed_at IS NOT NULL AND closed_at < ?`, cutoff); err != nil {
		return err
	}
	var rotated sql.NullString
	if err := db.SQL.QueryRowContext(ctx, `SELECT last_rotated_at FROM security_pgp_metadata WHERE id = 1`).Scan(&rotated); err != nil && err != sql.ErrNoRows {
		return err
	}
	if rotated.Valid {
		rotatedAt, err := time.Parse(time.RFC3339, rotated.String)
		if err == nil && !now.UTC().Before(rotatedAt.Add(30*24*time.Hour)) {
			for _, filename := range []string{"pgp.priv.old.asc.enc", "pgp.pub.old.asc"} {
				if err := os.Remove(filepath.Join(securityKeyDirectory(configDir), filename)); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Server) securityCleanupLoop(stop <-chan struct{}) {
	for {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 30, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-stop:
			timer.Stop()
			return
		case <-timer.C:
		}
		if s.database == nil || s.database.SQL == nil {
			continue
		}
		if err := CleanupSecurityReports(context.Background(), s.database, s.cfg.Server.Security.Reports.RetentionDays, s.cfg.ConfigDir, time.Now()); err != nil {
			log.Printf("security.cleanup_failed")
		}
	}
}
