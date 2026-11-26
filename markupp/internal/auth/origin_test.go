package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func TestOrigin_GuardadaNoContexto_VoltaIgual(t *testing.T) {
	origin := auth.Origin{IP: "203.0.113.7", UserAgent: "Firefox/140"}

	got := auth.OriginFrom(auth.WithOrigin(context.Background(), origin))

	assert.Equal(t, origin, got)
}

func TestOrigin_ContextoSemOrigem_VoltaVazia(t *testing.T) {
	assert.Equal(t, auth.Origin{}, auth.OriginFrom(context.Background()))
}
