package notify_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
)

var agora = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func detalhesDeTeste() notify.InviteDetails {
	return notify.InviteDetails{
		Email: "nova@markupp.dev", Role: "member", TenantName: "Equipe A",
		InviterEmail: "admin@markupp.dev", ExpiresAt: agora.Add(7 * 24 * time.Hour),
	}
}

func TestInviteMessage_LevaOTokenEQuemConvidou(t *testing.T) {
	msg := notify.InviteMessage(detalhesDeTeste(), "tok-123", "convites@markupp.dev")

	assert.Equal(t, "convites@markupp.dev", msg.From)
	assert.Equal(t, "nova@markupp.dev", msg.To)
	assert.Contains(t, msg.Subject, "Equipe A")
	for _, trecho := range []string{"tok-123", "admin@markupp.dev", "Equipe A", "member", "2026-10-22 12:00 UTC"} {
		assert.Contains(t, msg.Body, trecho)
	}
}

func TestNextAttempt_DobraAPartirDeUmMinutoAteUmaHora(t *testing.T) {
	esperado := map[int]time.Duration{
		1: time.Minute, 2: 2 * time.Minute, 3: 4 * time.Minute, 4: 8 * time.Minute,
		5: 16 * time.Minute, 6: 32 * time.Minute, 7: time.Hour,
	}
	for tentativas, espera := range esperado {
		quando, repete := notify.NextAttempt(tentativas, agora)

		assert.True(t, repete, "tentativa %d", tentativas)
		assert.Equal(t, agora.Add(espera), quando, "tentativa %d", tentativas)
	}
}

func TestNextAttempt_OitavaFalha_Desiste(t *testing.T) {
	_, repete := notify.NextAttempt(8, agora)

	assert.False(t, repete)
}

// fakeMailer faz o papel do servidor de email: guarda cada mensagem e falha com err.
type fakeMailer struct {
	sent []notify.Message
	err  error
}

func (f *fakeMailer) Send(_ context.Context, msg notify.Message) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, msg)
	return nil
}

// fakeOutbox faz o papel da fila no banco: devolve a reserva combinada, ou fila
// vazia com empty, guarda o hash e o lease pedidos e cada resultado gravado.
type fakeOutbox struct {
	claim    notify.Claim
	empty    bool
	err      error
	hash     string
	lease    time.Duration
	recorded map[string]notify.Outcome
}

func (f *fakeOutbox) ClaimNextDue(_ context.Context, tokenHash string, lease time.Duration) (notify.Claim, bool, error) {
	if f.err != nil {
		return notify.Claim{}, false, f.err
	}
	f.hash, f.lease = tokenHash, lease
	return f.claim, !f.empty, nil
}

func (f *fakeOutbox) Record(_ context.Context, notificationID string, outcome notify.Outcome) error {
	if f.recorded == nil {
		f.recorded = map[string]notify.Outcome{}
	}
	f.recorded[notificationID] = outcome
	return nil
}

func reservaPendente(attempt int) *fakeOutbox {
	return &fakeOutbox{claim: notify.Claim{NotificationID: "n1", Attempt: attempt, Details: detalhesDeTeste(), Pending: true}}
}

func tokenFixo() (string, error) { return "tok-novo", nil }

func novoDeliverer(fila notify.Outbox, mailer notify.Mailer) *notify.Deliverer {
	return notify.NewDeliverer(fila, mailer, "convites@markupp.dev", tokenFixo, func() time.Time { return agora })
}

func TestDeliverNext_ConvitePendente_GravaOHashEEnviaOToken(t *testing.T) {
	mailer := &fakeMailer{}
	fila := reservaPendente(1)

	found, err := novoDeliverer(fila, mailer).DeliverNext(context.Background())

	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, auth.HashOpaqueToken("tok-novo"), fila.hash, "o banco recebe so o hash do token")
	assert.Greater(t, fila.lease, 2*time.Minute, "o lease cobre o pior caso de uma sessao SMTP")
	require.Len(t, mailer.sent, 1)
	assert.Contains(t, mailer.sent[0].Body, "tok-novo")
	assert.Equal(t, map[string]notify.Outcome{"n1": {Sent: true}}, fila.recorded)
}

func TestDeliverNext_SMTPFalha_RemarcaComEsperaCrescente(t *testing.T) {
	fila := reservaPendente(3)

	found, err := novoDeliverer(fila, &fakeMailer{err: errors.New("conexao recusada")}).DeliverNext(context.Background())

	require.NoError(t, err, "falha do SMTP vira nova tentativa, nao erro do laco")
	assert.True(t, found)
	outcome := fila.recorded["n1"]
	assert.False(t, outcome.Sent)
	assert.False(t, outcome.GiveUp)
	assert.Equal(t, agora.Add(4*time.Minute), outcome.RetryAt, "terceira falha espera 4 minutos")
	assert.Contains(t, outcome.Error, "conexao recusada")
}

func TestDeliverNext_SMTPFalhaNaOitava_Desiste(t *testing.T) {
	fila := reservaPendente(8)

	_, err := novoDeliverer(fila, &fakeMailer{err: errors.New("conexao recusada")}).DeliverNext(context.Background())

	require.NoError(t, err)
	assert.True(t, fila.recorded["n1"].GiveUp)
}

func TestDeliverNext_TentativaAlemDoLimite_DesisteSemEnviar(t *testing.T) {
	mailer := &fakeMailer{}
	fila := reservaPendente(9)

	_, err := novoDeliverer(fila, mailer).DeliverNext(context.Background())

	require.NoError(t, err)
	assert.True(t, fila.recorded["n1"].GiveUp, "notificador que cai antes de gravar o resultado nao reenvia para sempre")
	assert.Empty(t, mailer.sent)
}

func TestDeliverNext_ConviteForaDoEnvio_DesisteSemEnviar(t *testing.T) {
	mailer := &fakeMailer{}
	fila := &fakeOutbox{claim: notify.Claim{NotificationID: "n1", Attempt: 1}}

	_, err := novoDeliverer(fila, mailer).DeliverNext(context.Background())

	require.NoError(t, err)
	assert.True(t, fila.recorded["n1"].GiveUp)
	assert.Empty(t, mailer.sent)
}

func TestDeliverNext_FilaVazia_DevolveFalsoSemEnviar(t *testing.T) {
	mailer := &fakeMailer{}
	fila := &fakeOutbox{empty: true}

	found, err := novoDeliverer(fila, mailer).DeliverNext(context.Background())

	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, mailer.sent)
	assert.Empty(t, fila.recorded)
}

func TestDeliverNext_ErroDoBanco_DevolveOErro(t *testing.T) {
	falha := errors.New("banco fora")

	_, err := novoDeliverer(&fakeOutbox{err: falha}, &fakeMailer{}).DeliverNext(context.Background())

	assert.ErrorIs(t, err, falha)
}
