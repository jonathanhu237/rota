package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type Emailer interface {
	Send(ctx context.Context, msg Message) error
}

type Message struct {
	To      string
	Subject string
	Body    string
}

type TemplateData struct {
	To              string
	Name            string
	BaseURL         string
	Token           string
	Language        string
	Expiration      time.Duration
	NewEmailPartial string
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	TLSMode  string
}

type SMTPEmailer struct {
	host      string
	addr      string
	auth      smtp.Auth
	from      string
	fromLine  string
	tlsMode   smtpTLSMode
	tlsConfig *tls.Config
	dialer    net.Dialer
}

type LoggerEmailer struct {
	writer io.Writer
}

type smtpTLSMode string

const (
	smtpTLSModeStartTLS smtpTLSMode = "starttls"
	smtpTLSModeImplicit smtpTLSMode = "implicit"
	smtpTLSModeNone     smtpTLSMode = "none"
)

func NewSMTPEmailer(config SMTPConfig) (*SMTPEmailer, error) {
	address, err := mail.ParseAddress(config.From)
	if err != nil {
		return nil, err
	}

	tlsMode, err := parseSMTPTLSMode(config.TLSMode)
	if err != nil {
		return nil, err
	}
	if tlsMode == smtpTLSModeNone && config.User != "" {
		return nil, errors.New("SMTP_USER cannot be set when SMTP_TLS_MODE is none")
	}

	var auth smtp.Auth
	if config.User != "" {
		auth = smtp.PlainAuth("", config.User, config.Password, config.Host)
	}

	return &SMTPEmailer{
		host:     config.Host,
		addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		auth:     auth,
		from:     address.Address,
		fromLine: address.String(),
		tlsMode:  tlsMode,
		tlsConfig: &tls.Config{
			ServerName: config.Host,
		},
	}, nil
}

func (e *SMTPEmailer) Send(ctx context.Context, msg Message) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	client, err := e.dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := e.handshake(client); err != nil {
		return err
	}
	if err := e.deliver(client, msg); err != nil {
		return err
	}
	return client.Quit()
}

func NewLoggerEmailer(writer io.Writer) *LoggerEmailer {
	if writer == nil {
		writer = io.Discard
	}

	return &LoggerEmailer{writer: writer}
}

func (e *LoggerEmailer) Send(_ context.Context, msg Message) error {
	_, err := fmt.Fprintf(
		e.writer,
		"=== EMAIL (dev) ===\nTo: %s\nSubject: %s\n\n%s\n",
		msg.To,
		msg.Subject,
		msg.Body,
	)
	return err
}

func BuildInvitationMessage(data TemplateData) Message {
	return renderTemplate("invitation", data)
}

func BuildPasswordResetMessage(data TemplateData) Message {
	return renderTemplate("password_reset", data)
}

func BuildEmailChangeConfirmMessage(data TemplateData) Message {
	return renderTemplate("email_change_confirm", data)
}

func BuildEmailChangeNoticeMessage(data TemplateData) Message {
	return renderTemplate("email_change_notice", data)
}

type localizedTemplate struct {
	subject string
	body    func(TemplateData) string
}

var templates = map[string]map[string]localizedTemplate{
	"en": {
		"invitation": {
			subject: "You've been invited to Rota",
			body:    invitationBody,
		},
		"password_reset": {
			subject: "Rota password reset",
			body:    passwordResetBody,
		},
		"email_change_confirm": {
			subject: "Confirm your email change",
			body:    emailChangeConfirmBody,
		},
		"email_change_notice": {
			subject: "Email change requested",
			body:    emailChangeNoticeBody,
		},
	},
	"zh": {
		"invitation": {
			subject: "You've been invited to Rota",
			body:    invitationBody,
		},
		"password_reset": {
			subject: "Rota password reset",
			body:    passwordResetBody,
		},
		"email_change_confirm": {
			subject: "确认邮箱变更",
			body:    emailChangeConfirmBody,
		},
		"email_change_notice": {
			subject: "邮箱变更请求",
			body:    emailChangeNoticeBody,
		},
	},
}

func renderTemplate(kind string, data TemplateData) Message {
	language := normalizeLanguage(data.Language)
	template := templates[language][kind]

	return Message{
		To:      data.To,
		Subject: template.subject,
		Body:    template.body(data),
	}
}

func normalizeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh":
		return "zh"
	default:
		return "en"
	}
}

func setupPasswordLink(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/setup-password?token=" + token
}

func emailChangeConfirmLink(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/auth/confirm-email-change?token=" + token
}

func invitationBody(data TemplateData) string {
	return fmt.Sprintf(
		"Hi %s,\n\nAn administrator has added you to Rota. Set your password here:\n%s\n\nThis link expires in %s.\n",
		data.Name,
		setupPasswordLink(data.BaseURL, data.Token),
		humanizeDuration(data.Expiration),
	)
}

func passwordResetBody(data TemplateData) string {
	return fmt.Sprintf(
		"Hi %s,\n\nSomeone requested a password reset for your account. If this was you, use this link:\n%s\n\nThis link expires in %s.\nIf this was not you, you can ignore this email.\n",
		data.Name,
		setupPasswordLink(data.BaseURL, data.Token),
		humanizeDuration(data.Expiration),
	)
}

func emailChangeConfirmBody(data TemplateData) string {
	return fmt.Sprintf(
		"Hi %s,\n\nConfirm your Rota email change here:\n%s\n\nThis link expires in %s.\nIf this was not you, you can ignore this email.\n",
		data.Name,
		emailChangeConfirmLink(data.BaseURL, data.Token),
		humanizeDuration(data.Expiration),
	)
}

func emailChangeNoticeBody(data TemplateData) string {
	return fmt.Sprintf(
		"Hi %s,\n\nAn email-change request was made for your Rota account. The requested new address is %s.\n\nIf this was not you, change your password immediately.\n",
		data.Name,
		data.NewEmailPartial,
	)
}

func PartialMaskEmail(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "@")
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}

	local := []rune(parts[0])
	if len(local) == 0 {
		return "***@" + parts[1]
	}
	return string(local[0]) + "***@" + parts[1]
}

func humanizeDuration(duration time.Duration) string {
	if duration%time.Hour == 0 {
		hours := int(duration / time.Hour)
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}

	if duration%time.Minute == 0 {
		minutes := int(duration / time.Minute)
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}

	return duration.String()
}

func parseSMTPTLSMode(value string) (smtpTLSMode, error) {
	switch value {
	case "", string(smtpTLSModeStartTLS):
		return smtpTLSModeStartTLS, nil
	case string(smtpTLSModeImplicit):
		return smtpTLSModeImplicit, nil
	case string(smtpTLSModeNone):
		return smtpTLSModeNone, nil
	default:
		return "", fmt.Errorf("invalid SMTP TLS mode %q", value)
	}
}

func (e *SMTPEmailer) dial(ctx context.Context) (*smtp.Client, error) {
	switch e.tlsMode {
	case smtpTLSModeImplicit:
		return e.dialTLS(ctx)
	default:
		return e.dialPlain(ctx)
	}
}

func (e *SMTPEmailer) dialPlain(ctx context.Context) (*smtp.Client, error) {
	conn, err := e.dialer.DialContext(ctx, "tcp", e.addr)
	if err != nil {
		return nil, err
	}

	client, err := smtp.NewClient(conn, e.host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return client, nil
}

func (e *SMTPEmailer) dialTLS(ctx context.Context) (*smtp.Client, error) {
	conn, err := e.dialer.DialContext(ctx, "tcp", e.addr)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, e.cloneTLSConfig())
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	client, err := smtp.NewClient(tlsConn, e.host)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}
	return client, nil
}

func (e *SMTPEmailer) handshake(client *smtp.Client) error {
	switch e.tlsMode {
	case smtpTLSModeStartTLS:
		if err := client.StartTLS(e.cloneTLSConfig()); err != nil {
			return err
		}
	case smtpTLSModeNone:
		return nil
	}

	if e.auth == nil {
		return nil
	}
	return client.Auth(e.auth)
}

func (e *SMTPEmailer) deliver(client *smtp.Client, msg Message) error {
	payload := strings.Join([]string{
		fmt.Sprintf("From: %s", e.fromLine),
		fmt.Sprintf("To: %s", msg.To),
		fmt.Sprintf("Subject: %s", msg.Subject),
		"",
		msg.Body,
	}, "\r\n")

	if err := client.Mail(e.from); err != nil {
		return err
	}
	if err := client.Rcpt(msg.To); err != nil {
		return err
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := io.WriteString(writer, payload); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}

func (e *SMTPEmailer) cloneTLSConfig() *tls.Config {
	if e.tlsConfig == nil {
		return &tls.Config{ServerName: e.host}
	}
	return e.tlsConfig.Clone()
}
