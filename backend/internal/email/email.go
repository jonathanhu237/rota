package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

type Emailer interface {
	Send(ctx context.Context, msg Message) error
}

type Message struct {
	Kind     string
	To       string
	Subject  string
	Body     string
	HTMLBody string
}

const (
	KindUnknown                    = "unknown"
	KindInvitation                 = "invitation"
	KindPasswordReset              = "password_reset"
	KindEmailChangeConfirm         = "email_change_confirm"
	KindEmailChangeNotice          = "email_change_notice"
	KindShiftChangeRequestReceived = "shift_change_request_received"
	KindShiftChangeResolved        = "shift_change_resolved"
)

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

type smtpSession struct {
	client *smtp.Client
	conn   net.Conn
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

	session, err := e.dial(ctx)
	if err != nil {
		return smtpContextError(ctx, err)
	}
	defer session.client.Close()

	stopContextWatch := watchSMTPContext(ctx, session.conn)
	defer stopContextWatch()
	if err := applySMTPDeadline(ctx, session.conn); err != nil {
		return err
	}

	if err := e.handshake(session.client); err != nil {
		return smtpContextError(ctx, err)
	}
	if err := e.deliver(session.client, msg); err != nil {
		return smtpContextError(ctx, err)
	}
	if err := session.client.Quit(); err != nil {
		return smtpContextError(ctx, err)
	}
	return nil
}

func NewLoggerEmailer(writer io.Writer) *LoggerEmailer {
	if writer == nil {
		writer = io.Discard
	}

	return &LoggerEmailer{writer: writer}
}

func (e *LoggerEmailer) Send(_ context.Context, msg Message) error {
	hasHTML := "no"
	if msg.HTMLBody != "" {
		hasHTML = "yes"
	}
	_, err := fmt.Fprintf(
		e.writer,
		"=== EMAIL (dev) ===\nTo: %s\nSubject: %s\nHTML: %s\n\n%s\n",
		msg.To,
		msg.Subject,
		hasHTML,
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

func renderTemplate(kind string, data TemplateData) Message {
	return renderAccountMessage(kind, data)
}

func NormalizeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh", "zh-cn", "zh-hans", "zh-hans-cn", "zh-tw", "zh-hant", "zh-hant-tw":
		return "zh"
	case "en", "en-us", "en-gb":
		return "en"
	default:
		return "en"
	}
}

func normalizeLanguage(language string) string {
	return NormalizeLanguage(language)
}

func setupPasswordLink(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/setup-password?token=" + token
}

func emailChangeConfirmLink(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/auth/confirm-email-change?token=" + token
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

func (e *SMTPEmailer) dial(ctx context.Context) (*smtpSession, error) {
	switch e.tlsMode {
	case smtpTLSModeImplicit:
		return e.dialTLS(ctx)
	default:
		return e.dialPlain(ctx)
	}
}

func (e *SMTPEmailer) dialPlain(ctx context.Context) (*smtpSession, error) {
	conn, err := e.dialer.DialContext(ctx, "tcp", e.addr)
	if err != nil {
		return nil, err
	}

	stopContextWatch := watchSMTPContext(ctx, conn)
	defer stopContextWatch()
	if err := applySMTPDeadline(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}

	client, err := smtp.NewClient(conn, e.host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &smtpSession{client: client, conn: conn}, nil
}

func (e *SMTPEmailer) dialTLS(ctx context.Context) (*smtpSession, error) {
	conn, err := e.dialer.DialContext(ctx, "tcp", e.addr)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, e.cloneTLSConfig())
	stopContextWatch := watchSMTPContext(ctx, tlsConn)
	defer stopContextWatch()
	if err := applySMTPDeadline(ctx, tlsConn); err != nil {
		tlsConn.Close()
		return nil, err
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		tlsConn.Close()
		return nil, err
	}

	client, err := smtp.NewClient(tlsConn, e.host)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}
	return &smtpSession{client: client, conn: tlsConn}, nil
}

func watchSMTPContext(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
}

func applySMTPDeadline(ctx context.Context, conn net.Conn) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil
	}
	return conn.SetDeadline(deadline)
}

func smtpContextError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			return context.DeadlineExceeded
		}
	}
	return err
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
	payload, err := e.renderPayload(msg)
	if err != nil {
		return err
	}

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
	if _, err := writer.Write(payload); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}

func (e *SMTPEmailer) renderPayload(msg Message) ([]byte, error) {
	var body bytes.Buffer
	writeMessageHeaders(&body, map[string]string{
		"From":         e.fromLine,
		"To":           msg.To,
		"Subject":      mime.QEncoding.Encode("utf-8", msg.Subject),
		"MIME-Version": "1.0",
	})

	if msg.HTMLBody == "" {
		writeMessageHeaders(&body, map[string]string{
			"Content-Type":              "text/plain; charset=UTF-8",
			"Content-Transfer-Encoding": "8bit",
		})
		body.WriteString("\r\n")
		body.WriteString(normalizeCRLF(msg.Body))
		return body.Bytes(), nil
	}

	multipartWriter := multipart.NewWriter(&body)
	writeMessageHeaders(&body, map[string]string{
		"Content-Type": `multipart/alternative; boundary="` + multipartWriter.Boundary() + `"`,
	})
	body.WriteString("\r\n")

	if err := writeMIMEPart(multipartWriter, "text/plain; charset=UTF-8", msg.Body); err != nil {
		return nil, err
	}
	if err := writeMIMEPart(multipartWriter, "text/html; charset=UTF-8", msg.HTMLBody); err != nil {
		return nil, err
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func writeMessageHeaders(body *bytes.Buffer, headers map[string]string) {
	for _, key := range []string{
		"From",
		"To",
		"Subject",
		"MIME-Version",
		"Content-Type",
		"Content-Transfer-Encoding",
	} {
		value, ok := headers[key]
		if !ok {
			continue
		}
		body.WriteString(key)
		body.WriteString(": ")
		body.WriteString(value)
		body.WriteString("\r\n")
	}
}

func writeMIMEPart(writer *multipart.Writer, contentType string, value string) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "8bit")

	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = io.WriteString(part, normalizeCRLF(value))
	return err
}

func normalizeCRLF(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}

func (e *SMTPEmailer) cloneTLSConfig() *tls.Config {
	if e.tlsConfig == nil {
		return &tls.Config{ServerName: e.host}
	}
	return e.tlsConfig.Clone()
}
