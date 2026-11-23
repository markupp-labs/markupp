package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const serviceType = "kubernetes:core/v1:Service"

func serviceNamed(t *testing.T, monitor *fakeResourceMonitor, name string) resource.PropertyMap {
	t.Helper()
	for _, r := range monitor.byType(serviceType) {
		if r.Name == name {
			return r.Inputs
		}
	}
	require.Failf(t, "Service ausente", "esperado o Service %q", name)
	return nil
}

// Instalações existentes têm o Service da API na 8080; mudar a porta do Service faz o
// server-side apply somar a nova à antiga, e o preview recusa duas portas sem nome.
func TestNewMarkupp_ServiceDaAPI_MantemA8080EEncaminhaParaAPortaDoServidor(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	ports := at(t, serviceNamed(t, monitor, "markupp-api"), "spec", "ports").ArrayValue()
	require.Len(t, ports, 1)
	assert.Equal(t, float64(8080), at(t, ports[0].ObjectValue(), "port").NumberValue())
	assert.Equal(t, float64(6275), at(t, ports[0].ObjectValue(), "targetPort").NumberValue())
}

func TestNewMarkupp_RotaDaAPI_ApontaParaAPortaDoService(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	rules := at(t, routeNamed(t, monitor, "markupp-api"), "spec", "rules").ArrayValue()
	backend := at(t, rules[0].ObjectValue(), "backendRefs").ArrayValue()[0].ObjectValue()
	assert.Equal(t, float64(8080), at(t, backend, "port").NumberValue())
}
