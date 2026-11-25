package notify_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
)

// entregaLogada roda uma entrega com o log em JSON e devolve a linha registrada,
// já decodificada, e o texto cru para conferir o que não pode aparecer.
func entregaLogada(t *testing.T, fila *fakeOutbox, mailer *fakeMailer) (map[string]any, string) {
	t.Helper()
	var logs bytes.Buffer
	deliverer := notify.NewDeliverer(fila, mailer, "convites@markupp.dev", "", tokenFixo, func() time.Time { return agora }, slog.New(slog.NewJSONHandler(&logs, nil)))

	_, err := deliverer.DeliverNext(context.Background())

	require.NoError(t, err)
	var linha map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &linha), "uma linha de log por entrega, em JSON: %s", logs.String())
	return linha, logs.String()
}

func TestDeliverNext_Enviada_RegistraIdTipoTentativaEResultado(t *testing.T) {
	linha, _ := entregaLogada(t, reservaPendente(2), &fakeMailer{})

	assert.Equal(t, "INFO", linha["level"])
	assert.Equal(t, "n1", linha["notificacao"])
	assert.Equal(t, "invite", linha["tipo"])
	assert.EqualValues(t, 2, linha["tentativa"])
	assert.Equal(t, "enviada", linha["resultado"])
}

func TestDeliverNext_FalhaDoSMTP_RegistraONovoHorarioEOErroDoServidor(t *testing.T) {
	falha := errors.New(`deliver to "nova@markupp.dev" through email-smtp:587: 554 Access denied`)

	linha, cru := entregaLogada(t, reservaPendente(3), &fakeMailer{err: falha})

	assert.Equal(t, "WARN", linha["level"])
	assert.Equal(t, "nova_tentativa", linha["resultado"])
	assert.Equal(t, agora.Add(4*time.Minute).Format(time.RFC3339), linha["nova_tentativa_em"])
	assert.Contains(t, linha["err"], "554 Access denied")
	assert.NotContains(t, cru, "nova@markupp.dev", "o log nunca leva o endereco de quem recebe")
	assert.NotContains(t, cru, "tok-novo", "o log nunca leva o token")
}

func TestDeliverNext_OitavaFalha_RegistraADesistencia(t *testing.T) {
	linha, _ := entregaLogada(t, reservaPendente(8), &fakeMailer{err: errors.New("554 Access denied")})

	assert.Equal(t, "desistencia", linha["resultado"])
	assert.Contains(t, linha["err"], "554 Access denied")
}

func TestDeliverNext_PedidoFechado_RegistraADesistenciaComOMotivo(t *testing.T) {
	fila := &fakeOutbox{claim: notify.Claim{NotificationID: "n1", Kind: notify.KindPasswordReset, Attempt: 1}}

	linha, _ := entregaLogada(t, fila, &fakeMailer{})

	assert.Equal(t, "desistencia", linha["resultado"])
	assert.Equal(t, "password_reset", linha["tipo"])
	assert.Contains(t, linha["err"], "no longer open")
}
