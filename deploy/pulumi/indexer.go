package main

import (
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// kedaPollingSeconds é de quanto em quanto tempo o KEDA consulta o banco, e é a
	// latência esperada da indexação e do primeiro envio de email (ADR-0032).
	kedaPollingSeconds = 30
	pendingVaultsQuery = "SELECT markupp_pending_vaults()"
	// indexerMaxJobs limita os Jobs simultâneos, cada um com seu pool no banco: sem
	// teto o KEDA sobe até 100, o max_connections padrão do PostgreSQL.
	indexerMaxJobs = 5
	// Job que falha não volta pelo backoff: o cofre continua na fila e o próximo
	// poll cria outro Job. O prazo mata o Job travado e solta a reserva do cofre.
	indexerBackoffLimit     = 1
	indexerDeadlineSeconds  = 600
	indexerSucceededHistory = 3
	indexerFailedHistory    = 5
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
	return declareCustom(ctx, "keda.sh/v1alpha1", "ScaledJob", names.indexer, args, pulumi.Map{
		"pollingInterval":            pulumi.Int(kedaPollingSeconds),
		"maxReplicaCount":            pulumi.Int(indexerMaxJobs),
		"successfulJobsHistoryLimit": pulumi.Int(indexerSucceededHistory),
		"failedJobsHistoryLimit":     pulumi.Int(indexerFailedHistory),
		"jobTargetRef": pulumi.Map{
			"template":              indexerPodTemplate(names, args),
			"backoffLimit":          pulumi.Int(indexerBackoffLimit),
			"activeDeadlineSeconds": pulumi.Int(indexerDeadlineSeconds),
		},
		"triggers": pulumi.Array{pendingRowsTrigger(names, pendingVaultsQuery)},
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
// cofres pendentes e as notificações vencidas, e não a do dono do banco. Existe
// quando o indexador ou o notificador existem.
func declareScalerAuth(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, opts pulumi.ResourceOption) error {
	if !indexerEnabled(args) && args.Email == nil {
		return nil
	}
	return declareCustom(ctx, "keda.sh/v1alpha1", "TriggerAuthentication", names.scaler, args, pulumi.Map{
		"secretTargetRef": pulumi.Array{pulumi.StringMap{
			"parameter": pulumi.String("connection"),
			"name":      pulumi.String(names.dbSecret),
			"key":       pulumi.String(scalerURIKey),
		}},
	}, opts)
}

// pendingRowsTrigger pede uma réplica, ou um Job, por linha pendente que query conta,
// consultando o banco como markupp_scaler.
func pendingRowsTrigger(names resourceNames, query string) pulumi.Map {
	return pulumi.Map{
		"type": pulumi.String("postgresql"),
		"metadata": pulumi.StringMap{
			"query":            pulumi.String(query),
			"targetQueryValue": pulumi.String("1"),
		},
		"authenticationRef": pulumi.StringMap{"name": pulumi.String(names.scaler)},
	}
}
