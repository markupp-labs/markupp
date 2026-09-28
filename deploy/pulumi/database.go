package main

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// dbSecretKey é a chave com a URL de conexão. O CloudNativePG já grava a URL
// com esse nome no Secret <cluster>-app, e o banco externo segue o mesmo
// formato para a API ler de um lugar só.
const dbSecretKey = "uri"

// declareDatabase cria o banco ou o Secret do banco externo, e devolve o
// recurso de que a migração depende.
func declareDatabase(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	if args.Database.ExternalURL != nil {
		return declareExternalDatabase(ctx, names, args, parent)
	}
	return declareCloudNativePG(ctx, names, args, parent)
}

// declareExternalDatabase guarda a URL num Secret. O provider Kubernetes já
// trata stringData de Secret como segredo no estado.
func declareExternalDatabase(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	return corev1.NewSecret(ctx, names.dbSecret, &corev1.SecretArgs{
		Metadata:   namespaced(names.dbSecret, args.Namespace),
		StringData: pulumi.StringMap{dbSecretKey: args.Database.ExternalURL},
	}, parent)
}

func declareCloudNativePG(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	storage := pulumi.Map{"size": pulumi.String(args.Database.Size)}
	if args.Database.StorageClass != "" {
		storage["storageClass"] = pulumi.String(args.Database.StorageClass)
	}
	return apiextensions.NewCustomResource(ctx, names.database, &apiextensions.CustomResourceArgs{
		ApiVersion: pulumi.String("postgresql.cnpg.io/v1"),
		Kind:       pulumi.String("Cluster"),
		Metadata:   namespaced(names.database, args.Namespace),
		OtherFields: kubernetes.UntypedArgs{"spec": pulumi.Map{
			"instances": pulumi.Int(args.Database.Instances),
			"storage":   storage,
			"bootstrap": pulumi.Map{"initdb": pulumi.Map{"database": pulumi.String("markupp"), "owner": pulumi.String("markupp")}},
		}},
	}, parent)
}

func databaseURLEnv(names resourceNames) corev1.EnvVarArray {
	return corev1.EnvVarArray{corev1.EnvVarArgs{
		Name: pulumi.String("MARKUPP_DATABASE_URL"),
		ValueFrom: corev1.EnvVarSourceArgs{SecretKeyRef: corev1.SecretKeySelectorArgs{
			Name: pulumi.String(names.dbSecret),
			Key:  pulumi.String(dbSecretKey),
		}},
	}}
}

func namespaced(name, namespace string) metav1.ObjectMetaArgs {
	return metav1.ObjectMetaArgs{Name: pulumi.String(name), Namespace: pulumi.String(namespace)}
}
