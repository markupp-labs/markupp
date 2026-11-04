package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

// Sem desligar, o kubelet injeta em cada pod variáveis com o nome de cada Service do
// namespace, e o Service markupp-mcp vira MARKUPP_MCP_PORT=tcp://..., que o servidor
// MCP lê como a porta dele e recusa.
func TestNewMarkupp_TodoPod_NaoRecebeVariaveisDosServices(t *testing.T) {
	args := argsComPainel()
	args.MCP = argsComMCP().MCP
	args.Email = argsComEmail().Email

	monitor := deploy(t, args)

	templates := map[string]resource.PropertyMap{}
	for _, token := range []string{"kubernetes:apps/v1:Deployment", "kubernetes:apps/v1:StatefulSet", "kubernetes:batch/v1:Job"} {
		for _, r := range monitor.byType(token) {
			templates[r.Name] = at(t, r.Inputs, "spec", "template", "spec").ObjectValue()
		}
	}
	templates["markupp-indexer"] = at(t, monitor.single(t, scaledJobType), "spec", "jobTargetRef", "template", "spec").ObjectValue()

	assert.Len(t, templates, 7, "api, panel, mcp, notifier, db, migrate e indexer")
	for name, spec := range templates {
		links := spec["enableServiceLinks"]
		assert.True(t, links.IsBool() && !links.BoolValue(), "%s sem enableServiceLinks: false", name)
	}
}
