package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMarkupp_ComAdministradoresDaInstancia_APIRecebeALista(t *testing.T) {
	args := argsDeTeste()
	args.InstanceAdmins = []string{"ana@markupp.dev.br", "bia@markupp.dev.br"}

	monitor := deploy(t, args)

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.Equal(t, "ana@markupp.dev.br,bia@markupp.dev.br", envValue(t, api, "MARKUPP_INSTANCE_ADMINS"))
}

func TestNewMarkupp_SemAdministradoresDaInstancia_APINaoRecebeAVariavel(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.NotContains(t, envNames(api), "MARKUPP_INSTANCE_ADMINS")
}

func TestNewMarkupp_ChaveSecreta_GeradaNoSecretDoBancoELidaPelaAPI(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	secret := monitor.single(t, "kubernetes:core/v1:Secret")
	assert.Equal(t, senhaDeTeste, plain(at(t, secret, "stringData", "secret-key")).StringValue())
	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	var ref map[string]string
	for _, env := range api["env"].ArrayValue() {
		if env.ObjectValue()["name"].StringValue() == "MARKUPP_SECRET_KEY" {
			key := at(t, env.ObjectValue(), "valueFrom", "secretKeyRef").ObjectValue()
			ref = map[string]string{"name": key["name"].StringValue(), "key": key["key"].StringValue()}
		}
	}
	require.NotNil(t, ref, "a API cifra a chave do provedor de embedding com MARKUPP_SECRET_KEY (ADR-0057)")
	assert.Equal(t, map[string]string{"name": "markupp-db-app", "key": "secret-key"}, ref)
}
