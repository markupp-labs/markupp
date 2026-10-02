package main

import (
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	randomPasswordType = "random:index/randomPassword:RandomPassword"
	senhaDeTeste       = "senha-gerada"
)

// fakeResourceMonitor faz o papel do motor do Pulumi: aceita cada recurso
// declarado e guarda os inputs para o teste inspecionar. Para a senha
// aleatória, devolve um valor fixo no lugar do que o provider geraria.
type fakeResourceMonitor struct {
	mu        sync.Mutex
	resources []pulumi.MockResourceArgs
}

func (f *fakeResourceMonitor) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resources = append(f.resources, args)
	state := args.Inputs.Copy()
	if args.TypeToken == randomPasswordType {
		state["result"] = resource.NewStringProperty(senhaDeTeste)
	}
	return args.Name + "-id", state, nil
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
		Database:  DatabaseArgs{StorageClass: "longhorn-fast"},
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

func declareErr(args MarkuppArgs) error {
	return pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := NewMarkupp(ctx, "markupp", args)
		return err
	}, pulumi.WithMocks("markupp", "teste", &fakeResourceMonitor{}))
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

func plain(value resource.PropertyValue) resource.PropertyValue {
	for value.IsSecret() {
		value = value.SecretValue().Element
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

	err := declareErr(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace")
}

func TestNewMarkupp_SemBancoExterno_CriaPostgresNoNamespaceComVolume(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	db := monitor.single(t, "kubernetes:apps/v1:StatefulSet")
	assert.Equal(t, "markupp", at(t, db, "metadata", "namespace").StringValue())
	assert.Equal(t, 1.0, at(t, db, "spec", "replicas").NumberValue())
	assert.Equal(t, "postgres:17.9-alpine3.23", firstContainer(t, db)["image"].StringValue(),
		"mesma versão fixa do compose e do testcontainers")
	claim := at(t, db, "spec", "volumeClaimTemplates").ArrayValue()[0].ObjectValue()
	assert.Equal(t, "longhorn-fast", at(t, claim, "spec", "storageClassName").StringValue())
	assert.Equal(t, "10Gi", at(t, claim, "spec", "resources", "requests", "storage").StringValue())
}

func TestNewMarkupp_SemBancoExterno_GuardaSenhaGeradaEURLNoSecret(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	require.Len(t, monitor.byType(randomPasswordType), 1)
	secret := monitor.single(t, "kubernetes:core/v1:Secret")
	assert.Equal(t, senhaDeTeste, plain(at(t, secret, "stringData", "password")).StringValue())
	assert.Equal(t, "postgres://markupp:"+senhaDeTeste+"@markupp-db:5432/markupp?sslmode=disable",
		plain(at(t, secret, "stringData", "uri")).StringValue())
}

func TestNewMarkupp_SemBancoExterno_PostgresLeASenhaDoSecret(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	db := monitor.single(t, "kubernetes:apps/v1:StatefulSet")
	var ref resource.PropertyValue
	for _, env := range firstContainer(t, db)["env"].ArrayValue() {
		if env.ObjectValue()["name"].StringValue() == "POSTGRES_PASSWORD" {
			ref = at(t, env.ObjectValue(), "valueFrom", "secretKeyRef")
		}
	}
	require.True(t, ref.IsObject(), "esperado POSTGRES_PASSWORD vindo de Secret")
	assert.Equal(t, "markupp-db-app", ref.ObjectValue()["name"].StringValue())
	assert.Equal(t, "password", ref.ObjectValue()["key"].StringValue())
}

func TestNewMarkupp_ComBancoExterno_NaoCriaPostgresEGuardaURLEmSecret(t *testing.T) {
	args := argsDeTeste()
	args.Database.ExternalURL = pulumi.String("postgres://markupp@db.externo:5432/markupp")

	monitor := deploy(t, args)

	assert.Empty(t, monitor.byType("kubernetes:apps/v1:StatefulSet"))
	assert.Empty(t, monitor.byType(randomPasswordType))
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

func TestNewMarkupp_DeploymentDaAPI_LeOBancoDoSecret(t *testing.T) {
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

func TestNewMarkupp_EmissorAutoassinado_DispensaEmailEACME(t *testing.T) {
	args := argsDeTeste()
	args.CertificateIssuer = "selfSigned"
	args.AcmeEmail = ""

	monitor := deploy(t, args)

	issuer := monitor.single(t, "kubernetes:cert-manager.io/v1:Issuer")
	assert.True(t, at(t, issuer, "spec", "selfSigned").IsObject(), "esperado spec.selfSigned")
	assert.True(t, at(t, issuer, "spec", "acme").IsNull(), "emissor autoassinado nao fala com ACME")
}

func TestNewMarkupp_EmissorACMESemEmail_RetornaErroComOCampo(t *testing.T) {
	args := argsDeTeste()
	args.AcmeEmail = ""

	err := declareErr(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "acmeEmail")
	assert.Contains(t, err.Error(), "certificateIssuer")
}

func TestNewMarkupp_EmissorDesconhecido_RetornaErroComOValor(t *testing.T) {
	args := argsDeTeste()
	args.CertificateIssuer = "letsencrypt"

	err := declareErr(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "letsencrypt")
	assert.Contains(t, err.Error(), "selfSigned")
}

func TestNewMarkupp_ZeroReplicas_DeixaAAPISemPods(t *testing.T) {
	args := argsDeTeste()
	zero := 0
	args.APIReplicas = &zero

	monitor := deploy(t, args)

	api := monitor.single(t, "kubernetes:apps/v1:Deployment")
	assert.Equal(t, 0.0, at(t, api, "spec", "replicas").NumberValue(), "zero réplicas pedido não vira o padrão")
}

func gatewayDoCluster() *GatewayRefArgs {
	return &GatewayRefArgs{Name: "gateway-compartilhado", Namespace: "gateway", SectionName: "https-markupp"}
}

func TestNewMarkupp_GatewayExistente_NaoCriaGatewayNemIssuer(t *testing.T) {
	args := argsDeTeste()
	args.ExistingGateway = gatewayDoCluster()

	monitor := deploy(t, args)

	assert.Empty(t, monitor.byType("kubernetes:gateway.networking.k8s.io/v1:Gateway"))
	assert.Empty(t, monitor.byType("kubernetes:cert-manager.io/v1:Issuer"))
}

func TestNewMarkupp_GatewayExistente_PenduraOHTTPRouteNele(t *testing.T) {
	args := argsDeTeste()
	args.ExistingGateway = gatewayDoCluster()

	monitor := deploy(t, args)

	route := monitor.single(t, "kubernetes:gateway.networking.k8s.io/v1:HTTPRoute")
	parent := at(t, route, "spec", "parentRefs").ArrayValue()[0].ObjectValue()
	assert.Equal(t, "gateway-compartilhado", parent["name"].StringValue())
	assert.Equal(t, "gateway", parent["namespace"].StringValue())
	assert.Equal(t, "https-markupp", parent["sectionName"].StringValue())
}

func TestNewMarkupp_GatewayExistente_DispensaEmailACME(t *testing.T) {
	args := argsDeTeste()
	args.ExistingGateway = gatewayDoCluster()
	args.AcmeEmail = ""

	assert.NoError(t, declareErr(args), "o certificado é de quem administra o Gateway existente")
}

func TestNewMarkupp_GatewayExistenteSemNome_RetornaErroComOCampo(t *testing.T) {
	args := argsDeTeste()
	args.ExistingGateway = &GatewayRefArgs{Namespace: "gateway"}

	err := declareErr(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "existingGateway.name")
}
