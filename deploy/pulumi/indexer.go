package main

import (
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// indexerPollingSeconds é de quanto em quanto tempo o KEDA consulta o banco, e é
	// a latência de indexação esperada (ADR-0032).
	indexerPollingSeconds = 30
	pendingVaultsQuery    = "SELECT markupp_pending_vaults()"
)

// IndexerArgs liga o indexador léxico.
type IndexerArgs struct {
	// Enabled cria o ScaledJob do indexador, que exige o KEDA no cluster. Padrão
	// true, e false instala banco e API sem indexação.
	Enabled *bool `pulumi:"enabled,optional"`
}

func indexerEnabled(args MarkuppArgs) bool {
	return args.Indexer == nil || args.Indexer.Enabled == nil || *args.Indexer.Enabled
}

// declareIndexer cria o ScaledJob, quando ligado: o KEDA conta os cofres pendentes
// no banco e cria um Job de markupp index por cofre, e nenhum quando não há o que
// indexar.
func declareIndexer(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, opts pulumi.ResourceOption) error {
	if !indexerEnabled(args) {
		return nil
	}
	container := serverContainer(args, names)
	container.Args = pulumi.StringArray{pulumi.String("index")}
	template := corev1.PodTemplateSpecArgs{Spec: corev1.PodSpecArgs{
		RestartPolicy:   pulumi.String("Never"),
		SecurityContext: nonRootPod(),
		Containers:      corev1.ContainerArray{container},
	}}
	return declareCustom(ctx, "keda.sh/v1alpha1", "ScaledJob", names.indexer, args, pulumi.Map{
		"pollingInterval": pulumi.Int(indexerPollingSeconds),
		"jobTargetRef":    pulumi.Map{"template": template},
		"triggers":        pulumi.Array{pendingVaultsTrigger()},
	}, opts)
}

// pendingVaultsTrigger lê a conexão do env do contêiner do Job, o mesmo Secret que
// a API usa, e pede um Job por cofre pendente.
func pendingVaultsTrigger() pulumi.Map {
	return pulumi.Map{
		"type": pulumi.String("postgresql"),
		"metadata": pulumi.StringMap{
			"query":             pulumi.String(pendingVaultsQuery),
			"targetQueryValue":  pulumi.String("1"),
			"connectionFromEnv": pulumi.String("MARKUPP_DATABASE_URL"),
		},
	}
}
