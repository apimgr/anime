package server

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"html"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/netip"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/anime/src/config"
)

func (s *Server) contactRecipient(kind string) string {
	switch kind {
	case "privacy":
		if value := strings.TrimSpace(s.cfg.Server.Contact.DPO.Email); value != "" {
			return value
		}
		if value := strings.TrimSpace(s.cfg.Server.Contact.General.Email); value != "" {
			return value
		}
	case "security":
		if value := strings.TrimSpace(s.cfg.Server.Contact.Security.Email); value != "" {
			return value
		}
	case "abuse", "dmca":
		if value := strings.TrimSpace(s.cfg.Server.Contact.Abuse.Email); value != "" {
			return value
		}
		if value := strings.TrimSpace(s.cfg.Server.Contact.General.Email); value != "" {
			return value
		}
	}
	return strings.TrimSpace(s.cfg.Server.Contact.Admin.Email)
}

func (s *Server) sendContactNotification(rHost, kind, senderName, senderEmail, subject, body string) error {
	if signature, err := s.signSecurityNotice([]byte(body)); err == nil {
		body += "\n\nOpenPGP signature:\n" + string(signature)
	}
	to := s.contactRecipient(kind)
	toAddress, err := mail.ParseAddress(to)
	if err != nil || toAddress.Address != to {
		return fmt.Errorf("contact recipient is not configured")
	}
	settings := s.cfg.Server.Notifications.Email
	host := strings.TrimSpace(settings.SMTP.Host)
	port := settings.SMTP.Port
	if port <= 0 {
		port = 587
	}
	if host == "" {
		return s.sendSMTPMessage("", port, settings, toAddress.Address, rHost, senderName, senderEmail, subject, body)
	}
	return s.sendSMTPMessage(host, port, settings, toAddress.Address, rHost, senderName, senderEmail, subject, body)
}

func (s *Server) sendSMTPMessage(host string, port int, settings config.EmailConfig, to, requestHost, senderName, senderEmail, subject, body string) error {
	if s.smtpProbed && !s.smtpAvailable {
		return fmt.Errorf("email delivery unavailable")
	}
	if host == "" {
		for _, candidateHost := range s.smtpHostCandidates(requestHost) {
			for _, candidatePort := range []int{25, 465, 587} {
				if err := s.sendSMTPMessage(candidateHost, candidatePort, settings, to, requestHost, senderName, senderEmail, subject, body); err == nil {
					return nil
				}
			}
		}
		return fmt.Errorf("email delivery unavailable")
	}
	if strings.ContainsAny(subject+senderName+senderEmail+to, "\r\n") {
		return fmt.Errorf("invalid email header")
	}
	from := strings.TrimSpace(settings.From.Email)
	if from == "" {
		from = "no-reply@" + s.contactSenderDomain(requestHost)
	}
	fromAddress, err := mail.ParseAddress(from)
	if err != nil || fromAddress.Address != from {
		return fmt.Errorf("configured sender is invalid")
	}
	fromName := strings.TrimSpace(settings.From.Name)
	if fromName == "" {
		fromName = "Anime Quotes API"
	}
	if strings.ContainsAny(fromName, "\r\n") {
		return fmt.Errorf("configured sender is invalid")
	}
	message, messageID, err := buildContactEmail(fromAddress.Address, fromName, to, senderEmail, subject, body, s.contactSenderDomain(requestHost))
	if err != nil {
		return err
	}
	_ = messageID
	address := net.JoinHostPort(host, fmt.Sprint(port))
	mode := strings.ToLower(strings.TrimSpace(settings.SMTP.TLS))
	if mode == "" {
		mode = "auto"
	}
	if !oneOf(mode, "auto", "starttls", "tls", "none") || port < 1 || port > 65535 || !validRequestHost(host) {
		return fmt.Errorf("email delivery configuration is invalid")
	}
	conn, err := dialSMTP(address, host, port, mode)
	if err != nil {
		return fmt.Errorf("email delivery failed")
	}
	defer closeAndLog(conn, "SMTP connection")
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("email delivery failed")
	}
	defer closeAndLog(client, "SMTP client")
	if mode == "auto" && port != 465 || mode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(smtpTLSConfig(host)); err != nil {
				return fmt.Errorf("email delivery failed")
			}
		} else if mode == "starttls" || settings.SMTP.Username != "" {
			return fmt.Errorf("email delivery failed")
		}
	}
	if settings.SMTP.Username != "" {
		if _, ok := conn.(*tls.Conn); !ok {
			if _, ok := client.TLSConnectionState(); !ok {
				return fmt.Errorf("email delivery failed")
			}
		}
		if err := client.Auth(smtp.PlainAuth("", settings.SMTP.Username, settings.SMTP.Password, host)); err != nil {
			return fmt.Errorf("email delivery failed")
		}
	}
	if err := client.Mail(fromAddress.Address); err != nil {
		return fmt.Errorf("email delivery failed")
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("email delivery failed")
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("email delivery failed")
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("email delivery failed")
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("email delivery failed")
	}
	_ = client.Quit()
	return nil
}

func (s *Server) smtpHostCandidates(requestHost string) []string {
	candidates := []string{"127.0.0.1", "172.17.0.1"}
	if host := defaultGatewayIPv4(); host != "" {
		candidates = append(candidates, host)
	}
	for _, host := range []string{s.cfg.Server.FQDN} {
		if host = normalizeContactHost(strings.TrimSpace(host)); host != "" {
			candidates = append(candidates, host)
		}
	}
	if host := globalIPv4(); host != "" {
		candidates = append(candidates, host)
	}
	for _, host := range []string{s.cfg.Server.FQDN} {
		if host != "" {
			candidates = append(candidates, "mail."+host, "smtp."+host)
		}
	}
	seen := make(map[string]bool, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, host := range candidates {
		if !seen[strings.ToLower(host)] {
			seen[strings.ToLower(host)] = true
			unique = append(unique, host)
		}
	}
	return unique
}

func (s *Server) initializeSMTP() {
	settings := s.cfg.Server.Notifications.Email
	host := strings.TrimSpace(settings.SMTP.Host)
	port := settings.SMTP.Port
	if port <= 0 {
		port = 587
	}
	s.smtpProbed = true
	if host != "" {
		s.smtpAvailable = probeSMTP(host, port, settings)
		if !s.smtpAvailable {
			log.Printf("email.smtp_unavailable host=%q port=%d", host, port)
		} else if address, err := netip.ParseAddr(host); err == nil && privateSMTPAddress(address) {
			log.Printf("SMTP host %s is a private address: TLS certificate verification skipped", host)
		}
		return
	}
	for _, candidate := range s.smtpHostCandidates(s.cfg.Server.FQDN) {
		for _, candidatePort := range []int{25, 465, 587} {
			if probeSMTP(candidate, candidatePort, settings) {
				s.cfg.Server.Notifications.Email.SMTP.Host = candidate
				s.cfg.Server.Notifications.Email.SMTP.Port = candidatePort
				s.smtpAvailable = true
				log.Printf("email.smtp_detected host=%q port=%d", candidate, candidatePort)
				if err := config.SaveCurrent(); err != nil {
					log.Printf("email.smtp_config_save_failed")
				}
				if address, err := netip.ParseAddr(candidate); err == nil && privateSMTPAddress(address) {
					log.Printf("SMTP host %s is a private address: TLS certificate verification skipped", candidate)
				}
				return
			}
		}
	}
	log.Printf("email.smtp_unavailable")
}

func probeSMTP(host string, port int, settings config.EmailConfig) bool {
	mode := strings.ToLower(strings.TrimSpace(settings.SMTP.TLS))
	if mode == "" {
		mode = "auto"
	}
	if port < 1 || port > 65535 || !validRequestHost(host) || !oneOf(mode, "auto", "tls", "starttls", "none") {
		return false
	}
	conn, err := dialSMTP(net.JoinHostPort(host, strconv.Itoa(port)), host, port, mode)
	if err != nil {
		return false
	}
	defer closeAndLog(conn, "SMTP probe connection")
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return false
	}
	defer closeAndLog(client, "SMTP probe client")
	isTLS := false
	if _, ok := client.TLSConnectionState(); ok {
		isTLS = true
	}
	if mode == "auto" && port != 465 || mode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(smtpTLSConfig(host)); err != nil {
				return false
			}
			isTLS = true
		} else if mode == "starttls" || settings.SMTP.Username != "" {
			return false
		}
	}
	return settings.SMTP.Username == "" || isTLS
}

func defaultGatewayIPv4() string {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return ""
	}
	defer closeAndLog(file, "routing table")
	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 4 || fields[1] != "00000000" {
				continue
			}
			flags, err := strconv.ParseUint(fields[3], 16, 16)
			if err != nil || flags&1 == 0 {
				continue
			}
			gateway, err := strconv.ParseUint(fields[2], 16, 32)
			if err != nil {
				continue
			}
			bytes := make([]byte, 4)
			binary.LittleEndian.PutUint32(bytes, uint32(gateway))
			return net.IP(bytes).String()
		}
	}
	return ""
}

func globalIPv4() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, networkInterface := range interfaces {
		addresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			var ip net.IP
			switch value := address.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			ipv4 := ip.To4()
			if ipv4 == nil || ipv4.IsPrivate() || ipv4.IsLoopback() || !ipv4.IsGlobalUnicast() {
				continue
			}
			return ipv4.String()
		}
	}
	return ""
}

func (s *Server) contactSenderDomain(requestHost string) string {
	if host := normalizeContactHost(strings.TrimSpace(s.cfg.Server.FQDN)); host != "" {
		return host
	}
	host := normalizeContactHost(requestHost)
	for _, domain := range configuredDomains() {
		if hostMatchesDomainList(host, []string{domain}) {
			return host
		}
	}
	return "localhost"
}

func dialSMTP(address, host string, port int, mode string) (net.Conn, error) {
	timeout := 5 * time.Second
	if mode == "tls" || mode == "auto" && port == 465 {
		return tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", address, smtpTLSConfig(host))
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func smtpTLSConfig(host string) *tls.Config {
	config := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if address, err := netip.ParseAddr(host); err == nil && privateSMTPAddress(address) {
		config.InsecureSkipVerify = true
	}
	return config
}

func privateSMTPAddress(address netip.Addr) bool {
	if address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() {
		return true
	}
	return address.Is4() && netip.MustParsePrefix("100.64.0.0/10").Contains(address)
}

func buildContactEmail(from, fromName, to, replyTo, subject, body, fqdn string) ([]byte, string, error) {
	if strings.ContainsAny(from+fromName+to+replyTo+subject, "\r\n") {
		return nil, "", fmt.Errorf("invalid email header")
	}
	if _, err := mailAddress(replyTo); err != nil {
		return nil, "", err
	}
	fromHeader := (&mail.Address{Name: fromName, Address: from}).String()
	encodedSubject := mime.QEncoding.Encode("UTF-8", "[Anime Quotes API contact] "+subject)
	boundaryBytes := make([]byte, 18)
	if _, err := rand.Read(boundaryBytes); err != nil {
		return nil, "", err
	}
	boundary := fmt.Sprintf("anime-%x", boundaryBytes)
	messageIDBytes := make([]byte, 16)
	if _, err := rand.Read(messageIDBytes); err != nil {
		return nil, "", err
	}
	messageID := fmt.Sprintf("<%x@%s>", messageIDBytes, fqdn)
	if fqdn == "" {
		messageID = fmt.Sprintf("<%x@localhost>", messageIDBytes)
	}
	var message strings.Builder
	message.WriteString("From: " + fromHeader + "\r\n")
	message.WriteString("To: " + (&mail.Address{Address: to}).String() + "\r\n")
	message.WriteString("Reply-To: " + (&mail.Address{Address: replyTo}).String() + "\r\n")
	message.WriteString("Subject: " + encodedSubject + "\r\n")
	message.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	message.WriteString("Message-ID: " + messageID + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Auto-Submitted: auto-generated\r\n")
	message.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n\r\n")
	message.WriteString("--" + boundary + "\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	message.WriteString(wrapBase64(base64.StdEncoding.EncodeToString([]byte(body))))
	message.WriteString("--" + boundary + "\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	message.WriteString(wrapBase64(base64.StdEncoding.EncodeToString([]byte("<pre>" + html.EscapeString(body) + "</pre>"))))
	message.WriteString("--" + boundary + "--\r\n")
	return []byte(message.String()), messageID, nil
}

func wrapBase64(value string) string {
	var wrapped strings.Builder
	for len(value) > 76 {
		wrapped.WriteString(value[:76] + "\r\n")
		value = value[76:]
	}
	wrapped.WriteString(value + "\r\n")
	return wrapped.String()
}

func mailAddress(value string) (string, error) {
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return "", fmt.Errorf("invalid email address")
	}
	return address.Address, nil
}
