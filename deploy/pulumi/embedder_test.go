package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pvcType = "kubernetes:core/v1:PersistentVolumeClaim"

func TestNewMarkupp_Embedder_RodaEmbedLoopNaImagemDoEmbedderComAMesmaTag(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	embedder := deploymentNamed(t, monitor, "markupp-embedder")
	container := firstContainer(t, embedder)
	assert.Equal(t, "ghcr.io/markupp-labs/markupp-embedder:v1.1.0", container["image"].StringValue())
	assert.Equal(t, "embed-loop", container["args"].ArrayValue()[0].StringValue())
	assert.Equal(t, 1.0, at(t, embedder, "spec", "replicas").NumberValue())
	assert.Equal(t, "Recreate", at(t, embedder, "spec", "strategy", "type").StringValue(), "o volume RWO não monta em duas réplicas")
}

func TestNewMarkupp_Embedder_LeOBancoEAChaveSecretaSemFixarOProvedor(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	container := firstContainer(t, deploymentNamed(t, monitor, "markupp-embedder"))
	assert.Equal(t, []string{"MARKUPP_DATABASE_URL", "MARKUPP_SECRET_KEY"}, envNames(container),
		"sem MARKUPP_EMBEDDING_PROVIDER vale a escolha do painel (ADR-0057)")
	key := secretRefOf(t, container, "MARKUPP_SECRET_KEY")
	assert.Equal(t, "markupp-db-app", key["name"].StringValue())
	assert.Equal(t, "secret-key", key["key"].StringValue())
}

func TestNewMarkupp_Embedder_GuardaOModeloNumVolumeDoNamespace(t *testing.T) {
	monitor := deploy(t, argsDeTeste())

	pvc := monitor.single(t, pvcType)
	assert.Equal(t, "markupp-embedder-models", at(t, pvc, "metadata", "name").StringValue())
	assert.Equal(t, "ReadWriteOnce", at(t, pvc, "spec", "accessModes").ArrayValue()[0].StringValue())
	assert.Equal(t, "2Gi", at(t, pvc, "spec", "resources", "requests", "storage").StringValue())
	assert.Equal(t, "longhorn-fast", at(t, pvc, "spec", "storageClassName").StringValue())
	embedder := deploymentNamed(t, monitor, "markupp-embedder")
	mount := firstContainer(t, embedder)["volumeMounts"].ArrayValue()[0].ObjectValue()
	assert.Equal(t, "/var/lib/markupp/models", mount["mountPath"].StringValue())
	volume := at(t, embedder, "spec", "template", "spec", "volumes").ArrayValue()[0].ObjectValue()
	assert.Equal(t, mount["name"].StringValue(), volume["name"].StringValue())
	assert.Equal(t, "markupp-embedder-models", at(t, volume, "persistentVolumeClaim", "claimName").StringValue())
	assert.Equal(t, float64(imageUID), at(t, embedder, "spec", "template", "spec", "securityContext", "fsGroup").NumberValue())
}

func TestNewMarkupp_EmbedderImage_TrocaAImagemDerivada(t *testing.T) {
	args := argsDeTeste()
	args.EmbedderImage = "registro.local/embedder:dev"

	monitor := deploy(t, args)

	container := firstContainer(t, deploymentNamed(t, monitor, "markupp-embedder"))
	assert.Equal(t, "registro.local/embedder:dev", container["image"].StringValue())
}

func TestNewMarkupp_ImagemPorDigestSemEmbedderImage_RetornaErroComOValor(t *testing.T) {
	args := argsDeTeste()
	args.Image = "ghcr.io/markupp-labs/markupp@sha256:abc"

	err := declareErr(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedderImage")
	assert.Contains(t, err.Error(), "sha256:abc")
}

func TestEmbedderImageOf_DerivaDoRepositorioDoServidor(t *testing.T) {
	for imagem, esperada := range map[string]string{
		"ghcr.io/markupp-labs/markupp:1.0.0": "ghcr.io/markupp-labs/markupp-embedder:1.0.0",
		"localhost:5000/markupp:dev":         "localhost:5000/markupp-embedder:dev",
		"markupp":                            "markupp-embedder",
	} {
		derivada, err := embedderImageOf(MarkuppArgs{Image: imagem})
		require.NoError(t, err, imagem)
		assert.Equal(t, esperada, derivada, imagem)
	}
}

// secretRefOf devolve o secretKeyRef da variável name do contêiner.
func secretRefOf(t *testing.T, container resource.PropertyMap, name string) resource.PropertyMap {
	t.Helper()
	for _, env := range container["env"].ArrayValue() {
		if env.ObjectValue()["name"].StringValue() == name {
			return at(t, env.ObjectValue(), "valueFrom", "secretKeyRef").ObjectValue()
		}
	}
	require.Failf(t, "variavel ausente", "esperada %s no conteiner", name)
	return nil
}
