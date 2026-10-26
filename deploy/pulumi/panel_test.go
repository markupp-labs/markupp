package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	httpRouteType = "kubernetes:gateway.networking.k8s.io/v1:HTTPRoute"
	gatewayType   = "kubernetes:gateway.networking.k8s.io/v1:Gateway"
	painelDeTeste = "notas.markupp.dev.br"
)

func argsComPainel() MarkuppArgs {
	args := argsDeTeste()
	args.Panel = &PanelArgs{Image: "ghcr.io/markupp-labs/markupp-web:1.1.0", Host: painelDeTeste}
	return args
}

func routeNamed(t *testing.T, monitor *fakeResourceMonitor, name string) resource.PropertyMap {
	t.Helper()
	for _, r := range monitor.byType(httpRouteType) {
		if r.Name == name {
			return r.Inputs
		}
	}
	require.Failf(t, "HTTPRoute ausente", "esperado o HTTPRoute %q", name)
	return nil
}

func envNamed(t *testing.T, container resource.PropertyMap, name string) resource.PropertyMap {
	t.Helper()
	for _, env := range container["env"].ArrayValue() {
		if env.ObjectValue()["name"].StringValue() == name {
			return env.ObjectValue()
		}
	}
	require.Failf(t, "variavel ausente", "esperada a variavel %q", name)
	return nil
}

func TestNewMarkupp_SemPainel_NaoCriaPainelNemAvisaAAPI(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.NotContains(t, envNames(api), "MARKUPP_PANEL_URL")
	assert.Len(t, monitor.byType(httpRouteType), 1, "so a rota da API")
}

// Pulumi YAML não omite propriedade: a stack sem painel passa o bloco vazio.
func TestNewMarkupp_BlocoDePainelVazio_SobeSemPainel(t *testing.T) {
	args := argsDeTeste()
	args.Panel = &PanelArgs{}

	monitor := deploy(t, args)

	assert.Len(t, monitor.byType(httpRouteType), 1)
}

func TestNewMarkupp_PainelSemImagem_RetornaErroComOCampo(t *testing.T) {
	args := argsComPainel()
	args.Panel.Image = ""

	err := declareErr(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "panel.image")
	assert.Contains(t, err.Error(), painelDeTeste)
}

func TestNewMarkupp_ComPainel_UmaReplicaFalandoComAAPIPeloService(t *testing.T) {
	monitor := deploy(t, argsComPainel())

	painel := deploymentNamed(t, monitor, "markupp-panel")
	assert.Equal(t, 1.0, at(t, painel, "spec", "replicas").NumberValue(),
		"a renovação do token fica no processo, mais réplicas pedem afinidade no Gateway")
	container := firstContainer(t, painel)
	assert.Equal(t, "ghcr.io/markupp-labs/markupp-web:1.1.0", container["image"].StringValue())
	assert.Equal(t, "http://markupp-api:8080", envNamed(t, container, "MARKUPP_API_URL")["value"].StringValue())
}

// A página Agents do painel mostra os comandos de registro com a URL pública da API, onde o MCP fica em /mcp.
func TestNewMarkupp_ComPainel_DaAoPainelAURLPublicaDaInstancia(t *testing.T) {
	monitor := deploy(t, argsComPainel())

	container := firstContainer(t, deploymentNamed(t, monitor, "markupp-panel"))
	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.Equal(t, envNamed(t, api, "MARKUPP_PUBLIC_URL")["value"].StringValue(),
		envNamed(t, container, "MARKUPP_PUBLIC_URL")["value"].StringValue())
}

// Sem MCP a API não serve /mcp: o painel não recebe a URL do MCP e a página Agents avisa em vez de mostrar os trechos.
func TestNewMarkupp_ComPainelSemMCP_NaoDaAoPainelAURLDoMCP(t *testing.T) {
	monitor := deploy(t, argsComPainel())

	container := firstContainer(t, deploymentNamed(t, monitor, "markupp-panel"))
	assert.NotContains(t, envNames(container), "MARKUPP_MCP_URL")
}

func TestNewMarkupp_ComPainelEMCP_DaAoPainelAURLDoMCP(t *testing.T) {
	args := argsComPainel()
	args.MCP = argsComMCP().MCP

	monitor := deploy(t, args)

	container := firstContainer(t, deploymentNamed(t, monitor, "markupp-panel"))
	mcp := firstContainer(t, deploymentNamed(t, monitor, "markupp-mcp"))
	assert.Equal(t, envNamed(t, mcp, "MARKUPP_MCP_RESOURCE_URL")["value"].StringValue(),
		envNamed(t, container, "MARKUPP_MCP_URL")["value"].StringValue())
}

func TestNewMarkupp_ComPainel_LeOSecretKeyBaseDeUmSecret(t *testing.T) {
	monitor := deploy(t, argsComPainel())

	container := firstContainer(t, deploymentNamed(t, monitor, "markupp-panel"))
	ref := at(t, envNamed(t, container, "SECRET_KEY_BASE"), "valueFrom", "secretKeyRef")
	assert.Equal(t, "markupp-panel", at(t, ref.ObjectValue(), "name").StringValue())
}

func TestNewMarkupp_ComPainel_RoteiaOHostDoPainelParaOService(t *testing.T) {
	monitor := deploy(t, argsComPainel())

	route := routeNamed(t, monitor, "markupp-panel")
	assert.Equal(t, painelDeTeste, at(t, route, "spec", "hostnames").ArrayValue()[0].StringValue())
	backend := at(t, route, "spec", "rules").ArrayValue()[0].ObjectValue()["backendRefs"].ArrayValue()[0].ObjectValue()
	assert.Equal(t, "markupp-panel", backend["name"].StringValue())
}

func TestNewMarkupp_ComPainel_APIMandaOConsentimentoParaOPainel(t *testing.T) {
	monitor := deploy(t, argsComPainel())

	api := firstContainer(t, deploymentNamed(t, monitor, "markupp-api"))
	assert.Equal(t, "https://"+painelDeTeste, envNamed(t, api, "MARKUPP_PANEL_URL")["value"].StringValue())
}

func TestNewMarkupp_ComPainelEGatewayProprio_AbreListenerParaOHostDoPainel(t *testing.T) {
	monitor := deploy(t, argsComPainel())

	var hosts []string
	for _, listener := range at(t, monitor.single(t, gatewayType), "spec", "listeners").ArrayValue() {
		if h := listener.ObjectValue()["hostname"]; h.IsString() {
			hosts = append(hosts, h.StringValue())
		}
	}
	assert.ElementsMatch(t, []string{"markupp.dev.br", painelDeTeste}, hosts)
}

func TestNewMarkupp_ComPainelEGatewayExistente_PenduraOPainelNoListenerDele(t *testing.T) {
	args := argsComPainel()
	args.ExistingGateway = gatewayDoCluster()
	args.Panel.SectionName = "https-painel"

	monitor := deploy(t, args)

	parent := at(t, routeNamed(t, monitor, "markupp-panel"), "spec", "parentRefs").ArrayValue()[0].ObjectValue()
	assert.Equal(t, "gateway-compartilhado", parent["name"].StringValue())
	assert.Equal(t, "https-painel", parent["sectionName"].StringValue())
}

func TestNewMarkupp_ComPainelEEmailSemPanelURL_LinkDosEmailsVaiProPainel(t *testing.T) {
	args := argsComPainel()
	args.Email = argsComEmail().Email

	monitor := deploy(t, args)

	notifier := firstContainer(t, deploymentNamed(t, monitor, "markupp-notifier"))
	assert.Equal(t, "https://"+painelDeTeste, envNamed(t, notifier, "MARKUPP_PANEL_URL")["value"].StringValue())
}
