package email

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoggerEmailerSend(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	emailer := NewLoggerEmailer(&output)

	err := emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Test subject",
		Body:    "Line one\nLine two",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	written := output.String()
	for _, want := range []string{
		"=== EMAIL (dev) ===",
		"To: worker@example.com",
		"Subject: Test subject",
		"Line one",
		"Line two",
	} {
		if !strings.Contains(written, want) {
			t.Fatalf("expected output to contain %q, got %q", want, written)
		}
	}
}

func TestBuildInvitationMessage(t *testing.T) {
	t.Parallel()

	msg := BuildInvitationMessage(TemplateData{
		To:         "worker@example.com",
		Name:       "Worker",
		BaseURL:    "http://localhost:5173",
		Token:      "setup-token",
		Language:   "en",
		Expiration: 72 * time.Hour,
	})

	if msg.To != "worker@example.com" {
		t.Fatalf("expected recipient to match, got %q", msg.To)
	}
	if msg.Subject != "You've been invited to Rota" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	for _, want := range []string{
		"Hi Worker,",
		"http://localhost:5173/setup-password?token=setup-token",
		"72 hours",
	} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, msg.Body)
		}
	}
}

func TestBuildPasswordResetMessage(t *testing.T) {
	t.Parallel()

	msg := BuildPasswordResetMessage(TemplateData{
		To:         "worker@example.com",
		Name:       "Worker",
		BaseURL:    "https://app.example.com/base/",
		Token:      "reset-token",
		Language:   "en",
		Expiration: time.Hour,
	})

	if msg.Subject != "Rota password reset" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	for _, want := range []string{
		"https://app.example.com/base/setup-password?token=reset-token",
		"1 hour",
		"If this was not you, you can ignore this email.",
	} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, msg.Body)
		}
	}
}

func TestBuildEmailChangeConfirmMessage(t *testing.T) {
	t.Parallel()

	msg := BuildEmailChangeConfirmMessage(TemplateData{
		To:         "alice2@example.com",
		Name:       "Alice",
		BaseURL:    "https://app.example.com",
		Token:      "email-token",
		Language:   "en",
		Expiration: 24 * time.Hour,
	})

	if msg.To != "alice2@example.com" {
		t.Fatalf("expected recipient to match, got %q", msg.To)
	}
	if msg.Subject != "Confirm your email change" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	for _, want := range []string{
		"Hi Alice,",
		"https://app.example.com/auth/confirm-email-change?token=email-token",
		"24 hours",
	} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, msg.Body)
		}
	}
}

func TestBuildEmailChangeNoticeMessage(t *testing.T) {
	t.Parallel()

	msg := BuildEmailChangeNoticeMessage(TemplateData{
		To:              "alice@example.com",
		Name:            "Alice",
		Language:        "en",
		NewEmailPartial: PartialMaskEmail("alice2@example.com"),
	})

	if msg.To != "alice@example.com" {
		t.Fatalf("expected recipient to match, got %q", msg.To)
	}
	if msg.Subject != "Email change requested" {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	for _, want := range []string{
		"Hi Alice,",
		"a***@example.com",
		"change your password",
	} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, msg.Body)
		}
	}
	if strings.Contains(msg.Body, "/auth/confirm-email-change") || strings.Contains(msg.Body, "?token=") {
		t.Fatalf("notice body must not contain an actionable confirmation link: %q", msg.Body)
	}
}

func TestPartialMaskEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "standard", in: "alice@example.com", want: "a***@example.com"},
		{name: "single rune", in: "a@example.com", want: "a***@example.com"},
		{name: "invalid", in: "not-an-email", want: "***"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := PartialMaskEmail(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNewSMTPEmailerRejectsPlainAuthWithoutTLS(t *testing.T) {
	t.Parallel()

	_, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host:     "localhost",
		Port:     1025,
		User:     "mailer",
		Password: "secret",
		From:     "Rota <noreply@example.com>",
	}, "none"))
	if err == nil {
		t.Fatal("expected error for SMTP user in none TLS mode")
	}
}

func TestNewSMTPEmailerAcceptsSupportedTLSModes(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"starttls", "implicit"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			_, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
				Host: "localhost",
				Port: 587,
				From: "Rota <noreply@example.com>",
			}, mode))
			if err != nil {
				t.Fatalf("NewSMTPEmailer returned error for mode %q: %v", mode, err)
			}
		})
	}
}

func TestSMTPEmailerSendRequiresSTARTTLSByDefault(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{})
	defer server.Close()

	emailer, err := NewSMTPEmailer(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	})
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}

	err = emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Invitation",
		Body:    "Please finish setup.",
	})
	if err == nil {
		t.Fatal("expected Send to fail when STARTTLS is unavailable")
	}
}

func TestSMTPEmailerSendWithSTARTTLSSucceeds(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{advertiseStartTLS: true})
	defer server.Close()

	emailer, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	}, "starttls"))
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}
	trustServerCertificate(t, emailer, server.Certificate())

	err = emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Invitation",
		Body:    "Please finish setup.",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if !server.StartTLSUsed() {
		t.Fatal("expected SMTP session to upgrade with STARTTLS")
	}
	if body := server.LastMessage(); !strings.Contains(body, "Please finish setup.") {
		t.Fatalf("expected delivered message body, got %q", body)
	}
}

func TestSMTPEmailerSendWithImplicitTLSSucceeds(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{implicitTLS: true})
	defer server.Close()

	emailer, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	}, "implicit"))
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}
	trustServerCertificate(t, emailer, server.Certificate())

	err = emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Reset",
		Body:    "Reset instructions.",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if body := server.LastMessage(); !strings.Contains(body, "Reset instructions.") {
		t.Fatalf("expected delivered message body, got %q", body)
	}
}

func TestSMTPEmailerSendWithoutTLSDeliversWithoutAuth(t *testing.T) {
	t.Parallel()

	server := newSMTPTestServer(t, smtpTestServerOptions{})
	defer server.Close()

	emailer, err := NewSMTPEmailer(withTLSMode(SMTPConfig{
		Host: "localhost",
		Port: server.Port(),
		From: "Rota <noreply@example.com>",
	}, "none"))
	if err != nil {
		t.Fatalf("NewSMTPEmailer returned error: %v", err)
	}

	err = emailer.Send(context.Background(), Message{
		To:      "worker@example.com",
		Subject: "Local dev",
		Body:    "Mailpit relay",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if server.StartTLSUsed() {
		t.Fatal("expected none mode to skip STARTTLS")
	}
	if body := server.LastMessage(); !strings.Contains(body, "Mailpit relay") {
		t.Fatalf("expected delivered message body, got %q", body)
	}
}

type smtpTestServerOptions struct {
	advertiseStartTLS bool
	implicitTLS       bool
}

type smtpTestServer struct {
	t           *testing.T
	listener    net.Listener
	tlsConfig   *tls.Config
	certificate *x509.Certificate

	advertiseStartTLS bool

	done chan struct{}

	mu           sync.Mutex
	startTLSUsed bool
	messages     []string
}

func newSMTPTestServer(t *testing.T, options smtpTestServerOptions) *smtpTestServer {
	t.Helper()

	tlsConfig, certificate := mustGenerateServerTLSConfig(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	if options.implicitTLS {
		listener = tls.NewListener(listener, tlsConfig)
	}

	server := &smtpTestServer{
		t:                 t,
		listener:          listener,
		tlsConfig:         tlsConfig,
		certificate:       certificate,
		advertiseStartTLS: options.advertiseStartTLS,
		done:              make(chan struct{}),
	}

	go server.serve()

	return server
}

func (s *smtpTestServer) serve() {
	defer close(s.done)

	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	if err := s.handleConnection(conn); err != nil && !errors.Is(err, io.EOF) {
		s.t.Errorf("SMTP test server error: %v", err)
	}
}

func (s *smtpTestServer) handleConnection(conn net.Conn) error {
	reader := newSMTPTestReader(conn)
	writer := newSMTPTestWriter(conn)
	if err := writer.line("220 localhost ESMTP ready"); err != nil {
		return err
	}

	for {
		line, err := reader.line()
		if err != nil {
			return err
		}

		command := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(command, "EHLO "), strings.HasPrefix(command, "HELO "):
			if err := writer.greetingCapabilities(s.advertiseStartTLS); err != nil {
				return err
			}
		case command == "STARTTLS":
			if !s.advertiseStartTLS {
				if err := writer.line("454 TLS not available"); err != nil {
					return err
				}
				continue
			}

			if err := writer.line("220 Ready to start TLS"); err != nil {
				return err
			}

			tlsConn := tls.Server(conn, s.tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				return err
			}

			s.recordStartTLS()
			conn = tlsConn
			reader = newSMTPTestReader(conn)
			writer = newSMTPTestWriter(conn)
			s.advertiseStartTLS = false
		case strings.HasPrefix(command, "MAIL FROM:"):
			if err := writer.line("250 OK"); err != nil {
				return err
			}
		case strings.HasPrefix(command, "RCPT TO:"):
			if err := writer.line("250 OK"); err != nil {
				return err
			}
		case command == "DATA":
			if err := writer.line("354 End data with <CR><LF>.<CR><LF>"); err != nil {
				return err
			}
			message, err := reader.data()
			if err != nil {
				return err
			}
			s.recordMessage(message)
			if err := writer.line("250 Message accepted"); err != nil {
				return err
			}
		case command == "QUIT":
			return writer.line("221 Bye")
		default:
			return fmt.Errorf("unexpected SMTP command: %q", line)
		}
	}
}

func (s *smtpTestServer) Close() {
	s.t.Helper()

	_ = s.listener.Close()
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		s.t.Fatal("timed out waiting for SMTP test server shutdown")
	}
}

func (s *smtpTestServer) Port() int {
	s.t.Helper()

	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *smtpTestServer) Certificate() *x509.Certificate {
	s.t.Helper()

	return s.certificate
}

func (s *smtpTestServer) StartTLSUsed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.startTLSUsed
}

func (s *smtpTestServer) LastMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.messages) == 0 {
		return ""
	}

	return s.messages[len(s.messages)-1]
}

func (s *smtpTestServer) recordStartTLS() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.startTLSUsed = true
}

func (s *smtpTestServer) recordMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, message)
}

type smtpTestReader struct {
	conn   net.Conn
	buffer []byte
}

func newSMTPTestReader(conn net.Conn) *smtpTestReader {
	return &smtpTestReader{conn: conn}
}

func (r *smtpTestReader) line() (string, error) {
	if err := r.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return "", err
	}

	for {
		index := bytes.Index(r.buffer, []byte("\r\n"))
		if index >= 0 {
			line := string(r.buffer[:index])
			r.buffer = r.buffer[index+2:]
			return line, nil
		}

		chunk := make([]byte, 1024)
		n, err := r.conn.Read(chunk)
		if err != nil {
			return "", err
		}
		r.buffer = append(r.buffer, chunk[:n]...)
	}
}

func (r *smtpTestReader) data() (string, error) {
	var lines []string
	for {
		line, err := r.line()
		if err != nil {
			return "", err
		}
		if line == "." {
			return strings.Join(lines, "\r\n"), nil
		}
		lines = append(lines, line)
	}
}

type smtpTestWriter struct {
	conn net.Conn
}

func newSMTPTestWriter(conn net.Conn) *smtpTestWriter {
	return &smtpTestWriter{conn: conn}
}

func (w *smtpTestWriter) line(value string) error {
	if err := w.conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	_, err := w.conn.Write([]byte(value + "\r\n"))
	return err
}

func (w *smtpTestWriter) greetingCapabilities(includeStartTLS bool) error {
	lines := []string{"250-localhost"}
	if includeStartTLS {
		lines = append(lines, "250-STARTTLS")
	}
	lines = append(lines, "250 OK")

	for _, line := range lines {
		if err := w.line(line); err != nil {
			return err
		}
	}
	return nil
}

func mustGenerateServerTLSConfig(t *testing.T) (*tls.Config, *x509.Certificate) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate returned error: %v", err)
	}

	certificate, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatalf("ParseCertificate returned error: %v", err)
	}

	pemCertificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	pemKey, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey returned error: %v", err)
	}
	pemPrivateKey := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: pemKey})

	serverCertificate, err := tls.X509KeyPair(pemCertificate, pemPrivateKey)
	if err != nil {
		t.Fatalf("X509KeyPair returned error: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCertificate},
		MinVersion:   tls.VersionTLS12,
	}, certificate
}

func withTLSMode(config SMTPConfig, mode string) SMTPConfig {
	config.TLSMode = mode
	return config
}

func trustServerCertificate(t *testing.T, emailer *SMTPEmailer, certificate *x509.Certificate) {
	t.Helper()

	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(certificate)

	emailer.tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
		ServerName: "localhost",
	}
}
