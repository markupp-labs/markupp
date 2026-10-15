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
	for _, trecho := range []string{"tok-123", "admin@markupp.dev", "Equipe A", "member", "2026-10-22"} {
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

// fakeInvites faz o papel do banco na troca do hash: guarda o hash recebido e
// devolve os detalhes do convite, ou found falso para convite já respondido.
type fakeInvites struct {
	hash    string
	missing bool
}

func (f *fakeInvites) reissue(_ context.Context, tokenHash string) (notify.InviteDetails, bool, error) {
	f.hash = tokenHash
	if f.missing {
		return notify.InviteDetails{}, false, nil
	}
	return detalhesDeTeste(), true, nil
}

func tokenFixo() (string, error) { return "tok-novo", nil }

func novoDeliverer(mailer notify.Mailer) *notify.Deliverer {
	return notify.NewDeliverer(mailer, "convites@markupp.dev", tokenFixo, func() time.Time { return agora })
}

func TestDeliver_ConvitePendente_GravaOHashEEnviaOToken(t *testing.T) {
	mailer := &fakeMailer{}
	convites := &fakeInvites{}

	outcome, err := novoDeliverer(mailer).Deliver(context.Background(), notify.PendingInvite{InviteID: "c1"}, convites.reissue)

	require.NoError(t, err)
	assert.Equal(t, notify.Outcome{Sent: true}, outcome)
	assert.Equal(t, auth.HashOpaqueToken("tok-novo"), convites.hash, "o banco recebe so o hash do token")
	require.Len(t, mailer.sent, 1)
	assert.Contains(t, mailer.sent[0].Body, "tok-novo")
}

func TestDeliver_SMTPFalha_RemarcaComEsperaCrescente(t *testing.T) {
	mailer := &fakeMailer{err: errors.New("conexao recusada")}

	outcome, err := novoDeliverer(mailer).Deliver(context.Background(), notify.PendingInvite{InviteID: "c1", Attempts: 2}, (&fakeInvites{}).reissue)

	require.NoError(t, err, "falha do SMTP vira nova tentativa, nao erro do laco")
	assert.False(t, outcome.Sent)
	assert.False(t, outcome.GiveUp)
	assert.Equal(t, agora.Add(4*time.Minute), outcome.RetryAt, "terceira falha espera 4 minutos")
	assert.Contains(t, outcome.Error, "conexao recusada")
}

func TestDeliver_SMTPFalhaNaOitava_Desiste(t *testing.T) {
	mailer := &fakeMailer{err: errors.New("conexao recusada")}

	outcome, err := novoDeliverer(mailer).Deliver(context.Background(), notify.PendingInvite{InviteID: "c1", Attempts: 7}, (&fakeInvites{}).reissue)

	require.NoError(t, err)
	assert.True(t, outcome.GiveUp)
}

func TestDeliver_ConviteJaRespondido_DesisteSemEnviar(t *testing.T) {
	mailer := &fakeMailer{}

	outcome, err := novoDeliverer(mailer).Deliver(context.Background(), notify.PendingInvite{InviteID: "c1"}, (&fakeInvites{missing: true}).reissue)

	require.NoError(t, err)
	assert.True(t, outcome.GiveUp)
	assert.Empty(t, mailer.sent)
}

// fakeOutbox faz o papel da fila no banco: devolve, chamada a chamada, se havia
// notificação vencida, ou falha com err.
type fakeOutbox struct {
	due   []bool
	err   error
	calls int
}

func (f *fakeOutbox) DeliverNextDue(_ context.Context, _ notify.DeliverFunc) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	if len(f.due) == 0 {
		return false, nil
	}
	found := f.due[0]
	f.due = f.due[1:]
	return found, nil
}

func TestRunLoop_EsvaziaAFilaAntesDeEsperar(t *testing.T) {
	outbox := &fakeOutbox{due: []bool{true, true, false}}
	esperas := 0
	espera := func(context.Context) error {
		esperas++
		return context.Canceled
	}

	err := notify.RunLoop(context.Background(), outbox, novoDeliverer(&fakeMailer{}).Deliver, espera)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 3, outbox.calls)
	assert.Equal(t, 1, esperas)
}

func TestRunLoop_ErroDoBanco_DevolveOErro(t *testing.T) {
	falha := errors.New("banco fora")

	err := notify.RunLoop(context.Background(), &fakeOutbox{err: falha}, novoDeliverer(&fakeMailer{}).Deliver,
		func(context.Context) error { return nil })

	assert.ErrorIs(t, err, falha)
}
