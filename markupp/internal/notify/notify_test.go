package notify_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
)

var agora = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func detalhesDeTeste() notify.TokenDetails {
	return notify.TokenDetails{
		Email: "nova@markupp.dev", Role: "member", TenantName: "Equipe A",
		InviterEmail: "admin@markupp.dev", ExpiresAt: agora.Add(7 * 24 * time.Hour),
	}
}

func TestInviteMessage_LevaOTokenEQuemConvidou(t *testing.T) {
	msg := notify.InviteMessage(detalhesDeTeste(), "tok-123", "convites@markupp.dev", "")

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
	return &fakeOutbox{claim: notify.Claim{NotificationID: "n1", Kind: notify.KindInvite, Attempt: attempt, Details: detalhesDeTeste(), Pending: true}}
}

func tokenFixo() (string, error) { return "tok-novo", nil }

func novoDeliverer(fila notify.Outbox, mailer notify.Mailer) *notify.Deliverer {
	return notify.NewDeliverer(fila, mailer, "convites@markupp.dev", "", tokenFixo, func() time.Time { return agora })
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

func detalhesDaConta() notify.TokenDetails {
	return notify.TokenDetails{Email: "nova@markupp.dev", ExpiresAt: time.Date(2026, 10, 16, 13, 30, 0, 0, time.UTC)}
}

func TestVerifyEmailMessage_LevaOTokenEAValidadeComHora(t *testing.T) {
	msg := notify.VerifyEmailMessage(detalhesDaConta(), "tok-v", "contas@markupp.dev", "")

	assert.Equal(t, "nova@markupp.dev", msg.To)
	assert.Equal(t, "contas@markupp.dev", msg.From)
	assert.Contains(t, msg.Subject, "Confirme")
	assert.Contains(t, msg.Body, "tok-v")
	assert.Contains(t, msg.Body, "2026-10-16 13:30 UTC")
}

func TestVerifyEmailMessage_AvisaQuemNaoSeCadastrou(t *testing.T) {
	msg := notify.VerifyEmailMessage(detalhesDaConta(), "tok-v", "contas@markupp.dev", "")

	assert.Contains(t, msg.Body, "ignore", "quem nao criou a conta sabe que nao deve usar o token")
	assert.Contains(t, msg.Body, "logado", "o token so vale na conta que pediu")
}

func TestResetPasswordMessage_LevaOTokenEAvisaQuemNaoPediu(t *testing.T) {
	msg := notify.ResetPasswordMessage(detalhesDaConta(), "tok-r", "contas@markupp.dev", "")

	assert.Contains(t, msg.Subject, "senha")
	assert.Contains(t, msg.Body, "tok-r")
	assert.Contains(t, msg.Body, "2026-10-16 13:30 UTC")
	assert.Contains(t, msg.Body, "ignore", "quem nao pediu a troca sabe que pode ignorar")
}

func TestDeliverNext_CadaTipo_MontaAMensagemDele(t *testing.T) {
	assuntos := map[notify.Kind]string{
		notify.KindInvite:            "Convite",
		notify.KindEmailVerification: "Confirme",
		notify.KindPasswordReset:     "senha",
	}
	for tipo, assunto := range assuntos {
		mailer := &fakeMailer{}
		fila := &fakeOutbox{claim: notify.Claim{NotificationID: "n1", Kind: tipo, Attempt: 1, Details: detalhesDaConta(), Pending: true}}

		_, err := novoDeliverer(fila, mailer).DeliverNext(context.Background())

		require.NoError(t, err, tipo)
		require.Len(t, mailer.sent, 1, tipo)
		assert.Contains(t, mailer.sent[0].Subject, assunto, tipo)
	}
}

func TestDeliverNext_TipoDesconhecido_DesisteSemEnviar(t *testing.T) {
	mailer := &fakeMailer{}
	fila := &fakeOutbox{claim: notify.Claim{NotificationID: "n1", Kind: "sms", Attempt: 1, Details: detalhesDaConta(), Pending: true}}

	_, err := novoDeliverer(fila, mailer).DeliverNext(context.Background())

	require.NoError(t, err)
	assert.Empty(t, mailer.sent)
	assert.True(t, fila.recorded["n1"].GiveUp)
	assert.Contains(t, fila.recorded["n1"].Error, "sms")
}

func TestInviteMessage_SemPainel_FicaComoAntes(t *testing.T) {
	msg := notify.InviteMessage(detalhesDeTeste(), "tok-123", "convites@markupp.dev", "")

	esperado := "admin@markupp.dev convidou você para Equipe A no markupp, com o papel member.\n\n" +
		"Token do convite:\ntok-123\n\n" +
		"Use o token no cliente do markupp para aceitar. O convite vale até 2026-10-22 12:00 UTC.\n"
	assert.Equal(t, esperado, msg.Body)
}

type montaMensagem func(notify.TokenDetails, string, string, string) notify.Message

func TestMensagens_SemPainel_NaoLevamLink(t *testing.T) {
	for _, monta := range []montaMensagem{notify.InviteMessage, notify.VerifyEmailMessage, notify.ResetPasswordMessage} {
		msg := monta(detalhesDeTeste(), "tok", "contas@markupp.dev", "")

		assert.NotContains(t, msg.Body, "http")
	}
}

func TestMensagens_ComPainel_LevamOLinkAntesDoToken(t *testing.T) {
	caminhos := map[string]montaMensagem{
		"/invite_acceptance/new":  notify.InviteMessage,
		"/email_confirmation/new": notify.VerifyEmailMessage,
		"/password_reset/new":     notify.ResetPasswordMessage,
	}
	for caminho, monta := range caminhos {
		for _, base := range []string{"https://painel.markupp.dev", "https://painel.markupp.dev/"} {
			msg := monta(detalhesDeTeste(), "a+b/c=", "contas@markupp.dev", base)

			link := "https://painel.markupp.dev" + caminho + "?token=a%2Bb%2Fc%3D"
			require.Contains(t, msg.Body, link, base)
			assert.Less(t, strings.Index(msg.Body, link), strings.Index(msg.Body, ":\na+b/c=\n"), "o token em texto vem logo abaixo do link")
		}
	}
}

func TestDeliverNext_ComPainel_EnviaOLinkDoConvite(t *testing.T) {
	mailer := &fakeMailer{}
	deliverer := notify.NewDeliverer(reservaPendente(1), mailer, "convites@markupp.dev", "http://localhost:3999", tokenFixo, func() time.Time { return agora })

	_, err := deliverer.DeliverNext(context.Background())

	require.NoError(t, err)
	require.Len(t, mailer.sent, 1)
	assert.Contains(t, mailer.sent[0].Body, "http://localhost:3999/invite_acceptance/new?token=tok-novo")
}
