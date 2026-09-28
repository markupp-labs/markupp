package main

import (
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeResourceMonitor faz o papel do motor do Pulumi: aceita cada recurso
// declarado e guarda os inputs para o teste inspecionar.
type fakeResourceMonitor struct {
	mu        sync.Mutex
	resources []pulumi.MockResourceArgs
}

func (f *fakeResourceMonitor) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resources = append(f.resources, args)
	return args.Name + "-id", args.Inputs, nil
}

func (f *fakeResourceMonitor) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func (f *fakeResourceMonitor) byType(token string) []pulumi.MockResourceArgs {
	f.mu.Lock()
	defer f.mu.Unlock()
	var found []pulumi.MockResourceArgs
	for _, r := range f.resources {
		if r.TypeToken == token {
			found = append(found, r)
		}
	}
	return found
}

func (f *fakeResourceMonitor) single(t *testing.T, token string) resource.PropertyMap {
	t.Helper()
	found := f.byType(token)
	require.Len(t, found, 1, "esperado um recurso %s", token)
	return found[0].Inputs
}

func argsDeTeste() MarkuppArgs {
	return MarkuppArgs{
		Namespace: "markupp",
		Image:     "ghcr.io/markupp-labs/markupp:v1.1.0",
		Host:      "markupp.dev.br",
		AcmeEmail: "equipe@markupp.dev.br",
		Database:  DatabaseArgs{StorageClass: "longhorn-single"},
	}
}

func deploy(t *testing.T, args MarkuppArgs) *fakeResourceMonitor {
	t.Helper()
	monitor := &fakeResourceMonitor{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewMarkupp(ctx, "markupp", args)
		return err
	}, pulumi.WithMocks("markupp", "teste", monitor))
	require.NoError(t, err)
	return monitor
}

func at(t *testing.T, pm resource.PropertyMap, keys ...string) resource.PropertyValue {
	t.Helper()
	value := resource.NewObjectProperty(pm)
	for _, key := range keys {
		for value.IsSecret() {
			value = value.SecretValue().Element
		}
		require.True(t, value.IsObject(), "esperado objeto antes de %q, recebido %v", key, value)
		value = value.ObjectValue()[resource.PropertyKey(key)]
	}
	return value
}

func firstContainer(t *testing.T, pm resource.PropertyMap) resource.PropertyMap {
	t.Helper()
	containers := at(t, pm, "spec", "template", "spec", "containers").ArrayValue()
	require.NotEmpty(t, containers)
	return containers[0].ObjectValue()
}

func TestNewMarkupp_SemNamespace_RetornaErroComOCampo(t *testing.T) {
	args := argsDeTeste()
	args.Namespace = ""

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewMarkupp(ctx, "markupp", args)
		return err
	}, pulumi.WithMocks("markupp", "teste", &fakeResourceMonitor{}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace")
}

func TestNewMarkupp_SemBancoExterno_CriaClusterCloudNativePG(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	cluster := monitor.single(t, "kubernetes:postgresql.cnpg.io/v1:Cluster")
	assert.Equal(t, 2.0, at(t, cluster, "spec", "instances").NumberValue())
	assert.Equal(t, "longhorn-single", at(t, cluster, "spec", "storage", "storageClass").StringValue())
	assert.Equal(t, "markupp", at(t, cluster, "metadata", "namespace").StringValue())
}

func TestNewMarkupp_ComBancoExterno_NaoCriaClusterEGuardaURLEmSecret(t *testing.T) {
	args := argsDeTeste()
	args.Database.ExternalURL = pulumi.String("postgres://markupp@db.externo:5432/markupp")

	monitor := deploy(t, args)

	assert.Empty(t, monitor.byType("kubernetes:postgresql.cnpg.io/v1:Cluster"))
	secret := monitor.single(t, "kubernetes:core/v1:Secret")
	assert.Equal(t, "postgres://markupp@db.externo:5432/markupp",
		at(t, secret, "stringData", "uri").StringValue())
}

func TestNewMarkupp_JobDeMigracao_RodaOSubcomandoMigrateNaImagem(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	job := monitor.single(t, "kubernetes:batch/v1:Job")
	container := firstContainer(t, job)
	assert.Equal(t, "ghcr.io/markupp-labs/markupp:v1.1.0", container["image"].StringValue())
	assert.Equal(t, "migrate", container["args"].ArrayValue()[0].StringValue())
}

func TestNewMarkupp_DeploymentDaAPI_LeOBancoDoSecretDoCluster(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	api := monitor.single(t, "kubernetes:apps/v1:Deployment")
	container := firstContainer(t, api)
	env := container["env"].ArrayValue()[0].ObjectValue()
	assert.Equal(t, "MARKUPP_DATABASE_URL", env["name"].StringValue())
	ref := at(t, env, "valueFrom", "secretKeyRef")
	assert.Equal(t, "markupp-db-app", at(t, ref.ObjectValue(), "name").StringValue())
	assert.Equal(t, "uri", at(t, ref.ObjectValue(), "key").StringValue())
}

func TestNewMarkupp_DeploymentDaAPI_UsaDuasReplicasESondaHealthz(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	api := monitor.single(t, "kubernetes:apps/v1:Deployment")
	assert.Equal(t, 2.0, at(t, api, "spec", "replicas").NumberValue())
	container := firstContainer(t, api)
	assert.Equal(t, "/healthz", at(t, container, "readinessProbe", "httpGet", "path").StringValue())
}

func TestNewMarkupp_Entrada_RoteiaOHostPeloGatewayComTLS(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	route := monitor.single(t, "kubernetes:gateway.networking.k8s.io/v1:HTTPRoute")
	assert.Equal(t, "markupp.dev.br", at(t, route, "spec", "hostnames").ArrayValue()[0].StringValue())
	gateway := monitor.single(t, "kubernetes:gateway.networking.k8s.io/v1:Gateway")
	assert.Equal(t, "cilium", at(t, gateway, "spec", "gatewayClassName").StringValue())
	listeners := at(t, gateway, "spec", "listeners").ArrayValue()
	require.Len(t, listeners, 2, "esperado um listener http para o desafio ACME e um https")
	assert.Equal(t, "markupp-issuer",
		at(t, gateway, "metadata", "annotations").ObjectValue()["cert-manager.io/issuer"].StringValue())
}

func TestNewMarkupp_Issuer_UsaOEmailACMEInformado(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	issuer := monitor.single(t, "kubernetes:cert-manager.io/v1:Issuer")
	assert.Equal(t, "equipe@markupp.dev.br", at(t, issuer, "spec", "acme", "email").StringValue())
}
