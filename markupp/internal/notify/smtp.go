package notify

import (
	"context"
	"crypto/rand"
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

// SMTPMailer entrega por SMTP. smtp:// exige subir para TLS com STARTTLS, e smtps://
// abre a conexão já em TLS. Texto puro só com smtp://...?tls=none, para servidor
// local como o Mailpit do compose: o email leva o token do convite.
type SMTPMailer struct {
	addr        string
	host        string
	implicitTLS bool
	plaintext   bool
	auth        smtp.Auth
}

// NewSMTPMailer lê o servidor de rawURL, no formato smtp://usuario:senha@host:porta
// ou smtps://. Sem porta, vale 587 para smtp e 465 para smtps.
func NewSMTPMailer(rawURL string) (*SMTPMailer, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "smtp" && parsed.Scheme != "smtps") || parsed.Hostname() == "" {
		return nil, fmt.Errorf("servidor de email %q, esperado smtp://usuario:senha@host:porta ou smtps://", redacted(rawURL))
	}
	plaintext, err := allowsPlaintext(parsed)
	if err != nil {
		return nil, err
	}
	m := &SMTPMailer{host: parsed.Hostname(), implicitTLS: parsed.Scheme == "smtps", plaintext: plaintext}
	m.addr = net.JoinHostPort(m.host, portOrDefault(parsed))
	if user := parsed.User; user != nil {
		password, _ := user.Password()
		m.auth = smtp.PlainAuth("", user.Username(), password, m.host)
	}
	return m, nil
}

// allowsPlaintext lê o parâmetro tls da URL: ausente exige TLS, e none, só com
// smtp://, libera texto puro.
func allowsPlaintext(u *url.URL) (bool, error) {
	mode := u.Query().Get("tls")
	if mode == "" {
		return false, nil
	}
	if mode == "none" && u.Scheme == "smtp" {
		return true, nil
	}
	return false, fmt.Errorf("servidor de email %q com tls=%q, esperado sem o parametro ou tls=none com smtp://", u.Redacted(), mode)
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
	client, err := m.connect(ctx, senderDomain(msg.From))
	if err != nil {
		return fmt.Errorf("conectar ao servidor de email %s: %w", m.addr, err)
	}
	defer func() { _ = client.Close() }()
	if err := deliver(client, msg); err != nil {
		return fmt.Errorf("entregar a %q por %s: %w", msg.To, m.addr, err)
	}
	// O servidor já aceitou a mensagem no fim do DATA. Tratar a falha do QUIT como
	// falha da entrega reenviaria o convite com outro token e mataria o deste email.
	_ = client.Quit()
	return nil
}

// connect abre a sessão SMTP já apresentada com heloName, protegida e autenticada.
func (m *SMTPMailer) connect(ctx context.Context, heloName string) (*smtp.Client, error) {
	conn, err := m.dial(ctx)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := m.greet(client, heloName); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (m *SMTPMailer) dial(ctx context.Context) (net.Conn, error) {
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
	return conn, nil
}

// greet se apresenta com o domínio do remetente: o padrão do net/smtp é EHLO
// localhost, que os filtros de spam punem.
func (m *SMTPMailer) greet(client *smtp.Client, heloName string) error {
	if err := client.Hello(heloName); err != nil {
		return err
	}
	if err := m.secure(client); err != nil {
		return fmt.Errorf("proteger a conexao com %s: %w", m.addr, err)
	}
	return nil
}

// secure sobe para TLS e autentica. O net/smtp recusa senha em texto puro fora de
// localhost.
func (m *SMTPMailer) secure(client *smtp.Client) error {
	if err := m.startTLS(client); err != nil {
		return err
	}
	if m.auth == nil {
		return nil
	}
	return client.Auth(m.auth)
}

// startTLS sobe a conexão em texto para TLS. Servidor sem STARTTLS é recusado, e
// assim quem remove o anúncio do caminho não lê o token.
func (m *SMTPMailer) startTLS(client *smtp.Client) error {
	if m.implicitTLS || m.plaintext {
		return nil
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("servidor sem STARTTLS, esperado STARTTLS, smtps:// ou smtp://...?tls=none para servidor local")
	}
	return client.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12})
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
	if _, err := w.Write(encode(msg, senderDomain(msg.From))); err != nil {
		return err
	}
	return w.Close()
}

// senderDomain é o domínio do remetente, que assina o Message-ID e o EHLO.
func senderDomain(from string) string {
	return from[strings.LastIndex(from, "@")+1:]
}

// encode escreve os cabeçalhos e o corpo em UTF-8, com fim de linha CRLF. O
// Message-ID vai daqui porque relay de submissão, como o Postfix, não o acrescenta.
func encode(msg Message, domain string) []byte {
	headers := []string{
		"Message-ID: <" + rand.Text() + "@" + domain + ">",
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
