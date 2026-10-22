package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Pulumi YAML não omite propriedade: a stack sem Gateway existente passa o bloco vazio.
func TestNewMarkupp_BlocoDeGatewayExistenteVazio_CriaGatewayEIssuerProprios(t *testing.T) {
	args := argsDeTeste()
	args.ExistingGateway = &GatewayRefArgs{}

	monitor := deploy(t, args)

	assert.Len(t, monitor.byType("kubernetes:gateway.networking.k8s.io/v1:Gateway"), 1)
	assert.Len(t, monitor.byType("kubernetes:cert-manager.io/v1:Issuer"), 1)
}
