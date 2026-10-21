package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pulumi YAML não omite propriedade: a stack sem email passa o bloco vazio.
func TestNewMarkupp_BlocoDeEmailVazio_SobeSemNotificador(t *testing.T) {
	args := argsDeTeste()
	args.Email = &EmailArgs{}

	monitor := deploy(t, args)

	assert.Empty(t, monitor.byType(scaledObjectType))
	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.NotContains(t, envNames(api), "MARKUPP_INVITE_BY_EMAIL", "sem SMTP a API devolve o token do convite")
}

func TestNewMarkupp_EmailSemSMTP_RetornaErroComOCampo(t *testing.T) {
	args := argsDeTeste()
	args.Email = &EmailArgs{From: "convites@markupp.dev"}

	err := declareErr(args)

	require.Error(t, err, "bloco pela metade desligaria o email sem aviso")
	assert.Contains(t, err.Error(), "email.smtpUrl")
	assert.Contains(t, err.Error(), "convites@markupp.dev")
}
