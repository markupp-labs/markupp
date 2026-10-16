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
	if err := declareScalerAuth(ctx, names, args, opts); err != nil {
		return err
	}
	return declareCustom(ctx, "keda.sh/v1alpha1", "ScaledJob", names.indexer, args, pulumi.Map{
		"pollingInterval": pulumi.Int(indexerPollingSeconds),
		"jobTargetRef":    pulumi.Map{"template": indexerPodTemplate(names, args)},
		"triggers":        pulumi.Array{pendingVaultsTrigger(names)},
	}, opts)
}

// indexerPodTemplate só passa a URL do banco: o indexador não assina token.
func indexerPodTemplate(names resourceNames, args MarkuppArgs) corev1.PodTemplateSpecArgs {
	container := serverContainer(args, names)
	container.Args = pulumi.StringArray{pulumi.String("index")}
	container.Env = corev1.EnvVarArray{secretEnv("MARKUPP_DATABASE_URL", names.dbSecret, dbSecretKey)}
	return corev1.PodTemplateSpecArgs{Spec: corev1.PodSpecArgs{
		RestartPolicy:   pulumi.String("Never"),
		SecurityContext: nonRootPod(),
		Containers:      corev1.ContainerArray{container},
	}}
}

// declareScalerAuth entrega ao KEDA a URL do papel markupp_scaler, que só conta os
// cofres pendentes, e não a do dono do banco.
func declareScalerAuth(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, opts pulumi.ResourceOption) error {
	return declareCustom(ctx, "keda.sh/v1alpha1", "TriggerAuthentication", names.indexer, args, pulumi.Map{
		"secretTargetRef": pulumi.Array{pulumi.StringMap{
			"parameter": pulumi.String("connection"),
			"name":      pulumi.String(names.dbSecret),
			"key":       pulumi.String(scalerURIKey),
		}},
	}, opts)
}

// pendingVaultsTrigger pede um Job por cofre pendente.
func pendingVaultsTrigger(names resourceNames) pulumi.Map {
	return pulumi.Map{
		"type": pulumi.String("postgresql"),
		"metadata": pulumi.StringMap{
			"query":            pulumi.String(pendingVaultsQuery),
			"targetQueryValue": pulumi.String("1"),
		},
		"authenticationRef": pulumi.StringMap{"name": pulumi.String(names.indexer)},
	}
}
