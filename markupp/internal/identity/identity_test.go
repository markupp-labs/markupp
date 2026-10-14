package identity_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

func TestActorFrom_ContextoComAtor_DevolveOAtor(t *testing.T) {
	ator := identity.Actor{UserID: "u1", TenantID: "t1", Role: "admin"}

	got, ok := identity.ActorFrom(identity.WithActor(context.Background(), ator))

	assert.True(t, ok)
	assert.Equal(t, ator, got)
}

func TestActorFrom_ContextoSemAtor_DevolveFalso(t *testing.T) {
	_, ok := identity.ActorFrom(context.Background())

	assert.False(t, ok)
}

func TestActor_IsAdmin_SoParaPapelAdmin(t *testing.T) {
	assert.True(t, identity.Actor{Role: "admin"}.IsAdmin())
	assert.False(t, identity.Actor{Role: "member"}.IsAdmin())
	assert.False(t, identity.Actor{}.IsAdmin())
}
