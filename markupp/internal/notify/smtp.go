package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

const smtpTimeout = 30 * time.Second

// SMTPMailer entrega por SMTP. smtp:// sobe para TLS com STARTTLS quando o servidor
// oferece, e smtps:// abre a conexão já em TLS.
type SMTPMailer struct {
	addr        string
	host        string
	implicitTLS bool
	auth        smtp.Auth
}

// NewSMTPMailer lê o servidor de rawURL, no formato smtp://usuario:senha@host:porta
// ou smtps://. Sem porta, vale 587 para smtp e 465 para smtps.
func NewSMTPMailer(rawURL string) (*SMTPMailer, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "smtp" && parsed.Scheme != "smtps") || parsed.Hostname() == "" {
		return nil, fmt.Errorf("servidor de email %q, esperado smtp://usuario:senha@host:porta ou smtps://", redacted(rawURL))
	}
	m := &SMTPMailer{host: parsed.Hostname(), implicitTLS: parsed.Scheme == "smtps"}
	m.addr = net.JoinHostPort(m.host, portOrDefault(parsed))
	if user := parsed.User; user != nil {
		password, _ := user.Password()
		m.auth = smtp.PlainAuth("", user.Username(), password, m.host)
	}
	return m, nil
}

func portOrDefault(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "smtps" {
		return "465"
	}
	return "587"
}

func redacted(rawURL string) string {
	if parsed, err := url.Parse(rawURL); err == nil {
		return parsed.Redacted()
	}
	return "valor ilegivel"
}

// Send abre uma conexão por mensagem e entrega msg.
func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	client, err := m.connect(ctx)
	if err != nil {
		return fmt.Errorf("conectar ao servidor de email %s: %w", m.addr, err)
	}
	defer func() { _ = client.Close() }()
	if err := m.secure(client); err != nil {
		return fmt.Errorf("proteger a conexao com %s: %w", m.addr, err)
	}
	if err := deliver(client, msg); err != nil {
		return fmt.Errorf("entregar a %q por %s: %w", msg.To, m.addr, err)
	}
	// O servidor já aceitou a mensagem no fim do DATA. Tratar a falha do QUIT como
	// falha da entrega reenviaria o convite com outro token e mataria o deste email.
	_ = client.Quit()
	return nil
}

func (m *SMTPMailer) connect(ctx context.Context) (*smtp.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(smtpTimeout))
	if m.implicitTLS {
		conn = tls.Client(conn, &tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12})
	}
	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

// secure sobe para TLS quando a conexão começou em texto e o servidor oferece, e
// autentica. O net/smtp recusa senha em texto puro fora de localhost.
func (m *SMTPMailer) secure(client *smtp.Client) error {
	if ok, _ := client.Extension("STARTTLS"); ok && !m.implicitTLS {
		if err := client.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if m.auth == nil {
		return nil
	}
	return client.Auth(m.auth)
}

func deliver(client *smtp.Client, msg Message) error {
	if err := client.Mail(msg.From); err != nil {
		return err
	}
	if err := client.Rcpt(msg.To); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(encode(msg)); err != nil {
		return err
	}
	return w.Close()
}

// encode escreve os cabeçalhos e o corpo em UTF-8, com fim de linha CRLF.
func encode(msg Message) []byte {
	headers := []string{
		"From: " + msg.From,
		"To: " + msg.To,
		"Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject),
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 8bit",
	}
	body := strings.ReplaceAll(msg.Body, "\n", "\r\n")
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body)
}
