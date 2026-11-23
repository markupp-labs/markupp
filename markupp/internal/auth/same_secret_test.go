package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func TestSameSecret_SoAceitaOMesmoSegredo(t *testing.T) {
	assert.True(t, auth.SameSecret("segredo", "segredo"))
	assert.False(t, auth.SameSecret("segredo", "outro"))
	assert.False(t, auth.SameSecret("segredo", ""))
}
