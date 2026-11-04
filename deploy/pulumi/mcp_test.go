package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func argsComMCP() MarkuppArgs {
	args := argsDeTeste()
	args.MCP = &MCPArgs{Image: "ghcr.io/markupp-labs/markupp-mcp:1.1.0"}
	return args
}

func TestNewMarkupp_SemMCP_APINaoEncaminhaOMCP(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.NotContains(t, envNames(api), "MARKUPP_MCP_UPSTREAM_URL")
	assert.NotContains(t, envNames(api), "MARKUPP_OAUTH_RESOURCES")
}

// Pulumi YAML não omite propriedade: a stack sem MCP passa o bloco vazio.
func TestNewMarkupp_BlocoDeMCPVazio_SobeSemMCP(t *testing.T) {
	args := argsDeTeste()
	args.MCP = &MCPArgs{}

	monitor := deploy(t, args)

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.NotContains(t, envNames(api), "MARKUPP_MCP_UPSTREAM_URL")
}

func TestNewMarkupp_ComMCP_APIEncaminhaOMCPPeloService(t *testing.T) {
	monitor := deploy(t, argsComMCP())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.Equal(t, "http://markupp-mcp:3000", envNamed(t, api, "MARKUPP_MCP_UPSTREAM_URL")["value"].StringValue())
	ref := at(t, envNamed(t, api, "MARKUPP_OAUTH_RESOURCES"), "valueFrom", "secretKeyRef")
	assert.Equal(t, "markupp-mcp", at(t, ref.ObjectValue(), "name").StringValue())
}

func TestNewMarkupp_ComMCP_ResourceDaAPIEOMesmoQueOMCPAnuncia(t *testing.T) {
	monitor := deploy(t, argsComMCP())

	var secret map[string]string
	for _, s := range monitor.byType("kubernetes:core/v1:Secret") {
		if s.Name == "markupp-mcp" {
			secret = map[string]string{}
			for k, v := range plain(at(t, s.Inputs, "stringData")).ObjectValue() {
				secret[string(k)] = plain(v).StringValue()
			}
		}
	}
	require.NotNil(t, secret, "esperado o Secret markupp-mcp")
	assert.Equal(t, "https://markupp.dev.br/mcp="+secret["resource-secret"], secret["oauth-resources"])
}

func TestNewMarkupp_ComMCP_ServidorFalaComAAPIPeloService(t *testing.T) {
	monitor := deploy(t, argsComMCP())

	container := firstContainer(t, deploymentNamed(t, monitor, "markupp-mcp"))
	assert.Equal(t, "ghcr.io/markupp-labs/markupp-mcp:1.1.0", container["image"].StringValue())
	assert.Equal(t, "http://markupp-api:8080", envNamed(t, container, "MARKUPP_MCP_API_URL")["value"].StringValue())
	assert.Equal(t, "https://markupp.dev.br/mcp", envNamed(t, container, "MARKUPP_MCP_RESOURCE_URL")["value"].StringValue())
	ref := at(t, envNamed(t, container, "MARKUPP_MCP_RESOURCE_SECRET"), "valueFrom", "secretKeyRef")
	assert.Equal(t, "resource-secret", at(t, ref.ObjectValue(), "key").StringValue())
}
