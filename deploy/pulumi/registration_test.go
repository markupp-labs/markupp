package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMarkupp_RegistroLigadoComEmail_APIRecebeAVariavel(t *testing.T) {
	args := argsComEmail()
	args.RegistrationEnabled = true

	monitor := deploy(t, args)

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.Equal(t, "true", envValue(t, api, "MARKUPP_AUTH_REGISTRATION_ENABLED"))
}

func TestNewMarkupp_SemRegistro_APINaoRecebeAVariavel(t *testing.T) {
	monitor := deploy(t, argsComEmail())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.NotContains(t, envNames(api), "MARKUPP_AUTH_REGISTRATION_ENABLED")
}

func TestNewMarkupp_RegistroLigadoSemEmail_RetornaErroComOsCampos(t *testing.T) {
	args := argsDeTeste()
	args.RegistrationEnabled = true

	err := declareErr(args)

	require.Error(t, err, "sem SMTP nada prova o email da conta, e qualquer pessoa ocupa um email alheio")
	assert.Contains(t, err.Error(), "registrationEnabled")
	assert.Contains(t, err.Error(), "email")
}
