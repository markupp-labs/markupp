package notify_test

import (
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// servidorSMTPFalso fala o mínimo de SMTP para o teste ver o que o cliente manda: o
// EHLO, o MAIL FROM e a mensagem. Não oferece STARTTLS, e com fechaNoQuit derruba
// a conexão no QUIT sem responder, como um relay que cai depois de aceitar o DATA.
type servidorSMTPFalso struct {
	fechaNoQuit bool
	mu          sync.Mutex
	ehlo        string
	mailFrom    string
	mensagem    string
	listener    net.Listener
}

// sobe escuta numa porta livre e devolve a URL smtp:// do servidor, já com tls=none.
func (s *servidorSMTPFalso) sobe(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s.listener = listener
	t.Cleanup(func() { _ = listener.Close() })
	go s.atende()
	return "smtp://" + listener.Addr().String() + "?tls=none"
}

func (s *servidorSMTPFalso) atende() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.conversa(textproto.NewConn(conn))
	}
}

func (s *servidorSMTPFalso) conversa(c *textproto.Conn) {
	defer func() { _ = c.Close() }()
	_ = c.PrintfLine("220 falso")
	for {
		linha, err := c.ReadLine()
		if err != nil {
			return
		}
		if !s.responde(c, linha) {
			return
		}
	}
}

// responde trata um comando e devolve false quando a conexão acaba.
func (s *servidorSMTPFalso) responde(c *textproto.Conn, linha string) bool {
	verbo := strings.ToUpper(strings.SplitN(linha, " ", 2)[0])
	switch verbo {
	case "EHLO":
		s.grava(&s.ehlo, linha)
		_ = c.PrintfLine("250 falso")
	case "MAIL":
		s.grava(&s.mailFrom, linha)
		_ = c.PrintfLine("250 ok")
	case "DATA":
		s.recebe(c)
	case "QUIT":
		if !s.fechaNoQuit {
			_ = c.PrintfLine("221 tchau")
		}
		return false
	default:
		_ = c.PrintfLine("250 ok")
	}
	return true
}

func (s *servidorSMTPFalso) recebe(c *textproto.Conn) {
	_ = c.PrintfLine("354 manda")
	corpo, _ := io.ReadAll(c.DotReader())
	s.grava(&s.mensagem, string(corpo))
	_ = c.PrintfLine("250 aceita")
}

func (s *servidorSMTPFalso) grava(campo *string, valor string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	*campo = valor
}

func (s *servidorSMTPFalso) lido(campo *string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *campo
}
