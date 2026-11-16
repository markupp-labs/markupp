package instance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

func TestAdminsIncludes_EmailNaListaVerificado_Administra(t *testing.T) {
	admins := instance.Admins{"ana@exemplo.org"}

	assert.True(t, admins.Includes("Ana@Exemplo.ORG", true))
}

func TestAdminsIncludes_EmailNaoVerificado_NaoAdministra(t *testing.T) {
	admins := instance.Admins{"ana@exemplo.org"}

	assert.False(t, admins.Includes("ana@exemplo.org", false))
}

func TestAdminsIncludes_EmailForaDaLista_NaoAdministra(t *testing.T) {
	admins := instance.Admins{"ana@exemplo.org"}

	assert.False(t, admins.Includes("bia@exemplo.org", true))
}

func TestAdminsIncludes_ListaVazia_NinguemAdministra(t *testing.T) {
	assert.False(t, instance.Admins(nil).Includes("ana@exemplo.org", true))
}
