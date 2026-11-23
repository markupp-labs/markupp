package embedding_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
)

// supervisorQueFalha monta o supervisor com o provedor local sempre falhando ao abrir.
func supervisorQueFalha() supervisorDeTeste {
	d := novoSupervisor(embedding.Setting{Provider: "local", Revision: time.Unix(1, 0)}, true)
	d.opener.err = errors.New("sem rede")
	return d
}

// esperaETenta confere que nada abre um segundo antes de wait e que abre ao completar wait.
func esperaETenta(t *testing.T, d supervisorDeTeste, wait time.Duration) {
	t.Helper()
	before := len(d.opener.opened)
	d.clock.advance(wait - time.Second)
	d.sup.Refresh(context.Background())
	assert.Len(t, d.opener.opened, before, "antes de %s nao tenta de novo", wait)
	d.clock.advance(time.Second)
	d.sup.Refresh(context.Background())
	assert.Len(t, d.opener.opened, before+1, "ao completar %s tenta de novo", wait)
}

func TestRefresh_FalhasSeguidas_DobramAEspera(t *testing.T) {
	d := supervisorQueFalha()
	d.sup.Refresh(context.Background())

	esperaETenta(t, d, retryBaseDeTeste)
	esperaETenta(t, d, 2*retryBaseDeTeste)
	esperaETenta(t, d, 4*retryBaseDeTeste)

	assert.Equal(t, []string{"downloading", "error", "downloading", "error", "downloading", "error", "downloading", "error"}, d.status.states,
		"durante a espera o estado fica em erro sem alternar")
}

func TestRefresh_FalhasSeguidas_EsperaParaEmTrintaMinutos(t *testing.T) {
	d := supervisorQueFalha()
	d.sup.Refresh(context.Background())
	for wait := retryBaseDeTeste; wait < 30*time.Minute; wait *= 2 {
		esperaETenta(t, d, wait)
	}

	esperaETenta(t, d, 30*time.Minute)
	esperaETenta(t, d, 30*time.Minute)
}

func TestRefresh_AbriuDepoisDeFalhar_ZeraAEspera(t *testing.T) {
	d := supervisorQueFalha()
	d.sup.Refresh(context.Background())
	esperaETenta(t, d, retryBaseDeTeste)
	d.opener.err = nil
	esperaETenta(t, d, 2*retryBaseDeTeste)
	d.opener.err = errors.New("sem rede")
	d.source.setting.Revision = time.Unix(2, 0)
	d.sup.Refresh(context.Background())

	esperaETenta(t, d, retryBaseDeTeste)
}

func TestRefresh_EscolhaMudouDuranteAEspera_TentaNaHoraEZeraAEspera(t *testing.T) {
	d := supervisorQueFalha()
	d.sup.Refresh(context.Background())
	esperaETenta(t, d, retryBaseDeTeste)
	d.source.setting.Revision = time.Unix(2, 0)

	d.sup.Refresh(context.Background())

	assert.Len(t, d.opener.opened, 3, "a escolha nova tenta sem esperar")
	esperaETenta(t, d, retryBaseDeTeste)
}
