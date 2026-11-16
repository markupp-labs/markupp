package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

var limitesDeTeste = auth.InviteLimits{PerAccount: 20, PerTenant: 50}

func TestCheckInviteQuota_Regras(t *testing.T) {
	casos := []struct {
		nome     string
		cota     auth.InviteQuota
		byEmail  bool
		esperado error
	}{
		{"verificado abaixo dos limites", auth.InviteQuota{InviterVerified: true, ByInviter: 19, ByTenant: 49}, true, nil},
		{"conta no limite", auth.InviteQuota{InviterVerified: true, ByInviter: 20, ByTenant: 0}, true, auth.ErrInviteLimit},
		{"tenant no limite", auth.InviteQuota{InviterVerified: true, ByInviter: 0, ByTenant: 50}, true, auth.ErrInviteLimit},
		{"email nao verificado com envio por email", auth.InviteQuota{ByInviter: 0, ByTenant: 0}, true, auth.ErrEmailNotVerified},
		{"email nao verificado sem envio por email", auth.InviteQuota{ByInviter: 0, ByTenant: 0}, false, nil},
		{"sem envio por email o limite vale igual", auth.InviteQuota{ByInviter: 20}, false, auth.ErrInviteLimit},
	}
	for _, c := range casos {
		err := auth.CheckInviteQuota(c.cota, limitesDeTeste, c.byEmail)

		if c.esperado == nil {
			assert.NoError(t, err, c.nome)
			continue
		}
		assert.ErrorIs(t, err, c.esperado, c.nome)
	}
}

func TestCheckInviteQuota_Limite_MensagemTrazContagemELimite(t *testing.T) {
	err := auth.CheckInviteQuota(auth.InviteQuota{InviterVerified: true, ByInviter: 20}, limitesDeTeste, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "20")
	assert.Contains(t, err.Error(), "24h")
}

func TestIssueInvite_CotaEsgotada_NaoDevolveToken(t *testing.T) {
	repo := &fakeAdminRepository{byInviter: 20}
	svc := auth.NewAdminService(repo, defaultClock(), false, limitesDeTeste)

	issued, err := svc.IssueInvite(context.Background(), adminDeTeste, "nova@markupp.dev", "member")

	assert.ErrorIs(t, err, auth.ErrInviteLimit)
	assert.Empty(t, issued.Token)
}

func TestIssueInvite_ContaSemEmailVerificadoComEnvioPorEmail_Recusa(t *testing.T) {
	repo := &fakeAdminRepository{unverified: true}
	svc := auth.NewAdminService(repo, defaultClock(), true, limitesDeTeste)

	_, err := svc.IssueInvite(context.Background(), adminDeTeste, "nova@markupp.dev", "member")

	assert.ErrorIs(t, err, auth.ErrEmailNotVerified)
}
