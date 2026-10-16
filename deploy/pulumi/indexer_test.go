package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	scaledJobType   = "kubernetes:keda.sh/v1alpha1:ScaledJob"
	triggerAuthType = "kubernetes:keda.sh/v1alpha1:TriggerAuthentication"
)

func containerDoIndexador(t *testing.T, scaledJob resource.PropertyMap) resource.PropertyMap {
	t.Helper()
	containers := at(t, scaledJob, "spec", "jobTargetRef", "template", "spec", "containers").ArrayValue()
	require.NotEmpty(t, containers)
	return containers[0].ObjectValue()
}

func textoEm(t *testing.T, pm resource.PropertyMap, keys ...string) string {
	t.Helper()
	value := plain(at(t, pm, keys...))
	require.True(t, value.IsString(), "esperado texto em %v, recebido %v", keys, value)
	return value.StringValue()
}

func nomesDoEnv(container resource.PropertyMap) []string {
	var nomes []string
	for _, env := range container["env"].ArrayValue() {
		nomes = append(nomes, env.ObjectValue()["name"].StringValue())
	}
	return nomes
}

func TestNewMarkupp_IndexadorPorPadrao_CriaScaledJobQueRodaIndexNaImagem(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	scaledJob := monitor.single(t, scaledJobType)
	container := containerDoIndexador(t, scaledJob)
	assert.Equal(t, "ghcr.io/markupp-labs/markupp:v1.1.0", container["image"].StringValue())
	assert.Equal(t, "index", container["args"].ArrayValue()[0].StringValue())
	assert.Equal(t, "Never", at(t, scaledJob, "spec", "jobTargetRef", "template", "spec", "restartPolicy").StringValue())
}

func TestNewMarkupp_Indexador_PodNaoRecebeOSegredoDosJWT(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	container := containerDoIndexador(t, monitor.single(t, scaledJobType))
	assert.Equal(t, []string{"MARKUPP_DATABASE_URL"}, nomesDoEnv(container))
}

func TestNewMarkupp_Indexador_ScalerContaCofresPendentesComPapelProprio(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	scaledJob := monitor.single(t, scaledJobType)
	assert.Equal(t, 30.0, at(t, scaledJob, "spec", "pollingInterval").NumberValue())
	triggers := at(t, scaledJob, "spec", "triggers").ArrayValue()
	require.Len(t, triggers, 1)
	trigger := triggers[0].ObjectValue()
	assert.Equal(t, "postgresql", trigger["type"].StringValue())
	metadata := trigger["metadata"].ObjectValue()
	assert.Equal(t, "SELECT markupp_pending_vaults()", metadata["query"].StringValue())
	assert.Equal(t, "1", metadata["targetQueryValue"].StringValue(), "um Job por cofre pendente")
	assert.True(t, metadata["connectionFromEnv"].IsNull(), "a URL do Job e a do dono do banco")
	assert.Equal(t, "markupp-indexer", at(t, trigger, "authenticationRef", "name").StringValue())
}

func TestNewMarkupp_Indexador_TriggerAuthenticationLeAURLDoScaler(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	auth := monitor.single(t, triggerAuthType)
	assert.Equal(t, "markupp-indexer", at(t, auth, "metadata", "name").StringValue())
	refs := at(t, auth, "spec", "secretTargetRef").ArrayValue()
	require.Len(t, refs, 1)
	ref := refs[0].ObjectValue()
	assert.Equal(t, "connection", ref["parameter"].StringValue())
	assert.Equal(t, "markupp-db-app", ref["name"].StringValue())
	assert.Equal(t, "scaler-uri", ref["key"].StringValue())
}

func TestNewMarkupp_SemBancoExterno_GuardaURLDoScalerComHostCompleto(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	secret := monitor.single(t, "kubernetes:core/v1:Secret")
	assert.Equal(t, senhaDeTeste, textoEm(t, secret, "stringData", "scaler-password"))
	assert.Equal(t, "postgres://markupp_scaler:"+senhaDeTeste+"@markupp-db.markupp.svc:5432/markupp?sslmode=disable",
		textoEm(t, secret, "stringData", "scaler-uri"),
		"o scaler roda no namespace keda, onde o nome curto do Service nao resolve")
}

func TestNewMarkupp_ComBancoExterno_TrocaSoOUsuarioNaURLDoScaler(t *testing.T) {
	args := argsDeTeste()
	args.Database.ExternalURL = pulumi.String("postgres://dono:segredo@db.externo:5432/markupp?sslmode=require")

	monitor := deploy(t, args)

	secret := monitor.single(t, "kubernetes:core/v1:Secret")
	assert.Equal(t, "postgres://markupp_scaler:"+senhaDeTeste+"@db.externo:5432/markupp?sslmode=require",
		textoEm(t, secret, "stringData", "scaler-uri"))
}

func TestNewMarkupp_JobDeMigracao_RecebeASenhaDoScaler(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	container := firstContainer(t, monitor.single(t, "kubernetes:batch/v1:Job"))
	assert.Contains(t, nomesDoEnv(container), "MARKUPP_SCALER_PASSWORD", "a migracao da LOGIN ao papel do scaler")
}

func TestNewMarkupp_IndexadorDesligado_NaoCriaScaledJob(t *testing.T) {
	args := argsDeTeste()
	desligado := false
	args.Indexer = &IndexerArgs{Enabled: &desligado}

	monitor := deploy(t, args)

	assert.Empty(t, monitor.byType(scaledJobType), "cluster sem KEDA instala banco e API sem o indexador")
	assert.Empty(t, monitor.byType(triggerAuthType))
	monitor.single(t, "kubernetes:apps/v1:Deployment")
}
