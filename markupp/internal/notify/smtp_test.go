package notify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
)

const mailpitImage = "axllent/mailpit:v1.31.4"

// mailpit sobe um servidor SMTP de verdade, que guarda o que recebe e expõe as
// mensagens por HTTP, e devolve as URLs do SMTP e da API dele.
func mailpit(t *testing.T) (smtpURL, apiURL string) {
	t.Helper()
	ctx := context.Background()
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        mailpitImage,
			ExposedPorts: []string{"1025/tcp", "8025/tcp"},
			WaitingFor:   wait.ForHTTP("/livez").WithPort("8025/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err, "subir o mailpit de teste exige Docker disponivel")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })
	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	smtpPort, err := ctr.MappedPort(ctx, "1025")
	require.NoError(t, err)
	apiPort, err := ctr.MappedPort(ctx, "8025")
	require.NoError(t, err)
	return fmt.Sprintf("smtp://%s:%s", host, smtpPort.Port()), fmt.Sprintf("http://%s:%s", host, apiPort.Port())
}

type caixaDoMailpit struct {
	Messages []struct {
		Subject string
		To      []struct{ Address string }
	} `json:"messages"`
}

func TestSMTPMailer_Send_EntregaAoServidor(t *testing.T) {
	smtpURL, apiURL := mailpit(t)
	mailer, err := notify.NewSMTPMailer(smtpURL)
	require.NoError(t, err)

	err = mailer.Send(context.Background(), notify.InviteMessage(detalhesDeTeste(), "tok-123", "convites@markupp.dev"))
	require.NoError(t, err)

	resp, err := http.Get(apiURL + "/api/v1/messages")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var caixa caixaDoMailpit
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&caixa))
	require.Len(t, caixa.Messages, 1)
	assert.Contains(t, caixa.Messages[0].Subject, "Equipe A")
	assert.Equal(t, "nova@markupp.dev", caixa.Messages[0].To[0].Address)
}

func TestSMTPMailer_ServidorInalcancavel_RetornaErroComOEndereco(t *testing.T) {
	mailer, err := notify.NewSMTPMailer("smtp://127.0.0.1:1")
	require.NoError(t, err)

	err = mailer.Send(context.Background(), notify.InviteMessage(detalhesDeTeste(), "tok", "convites@markupp.dev"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "127.0.0.1:1")
}

func TestNewSMTPMailer_EsquemaDesconhecido_RetornaErroComOEsquema(t *testing.T) {
	_, err := notify.NewSMTPMailer("http://smtp.markupp.dev")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "http")
}
