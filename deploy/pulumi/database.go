package main

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// dbSecretKey é a chave com a URL de conexão que a API e a migração leem.
	dbSecretKey      = "uri"
	dbPasswordKey    = "password"
	postgresImage    = "postgres:17-alpine"
	postgresPort     = 5432
	postgresUser     = "markupp"
	passwordLength   = 32
	postgresDataPath = "/var/lib/postgresql/data"
)

// declareDatabase cria o Postgres do namespace ou o Secret do banco externo,
// e devolve o recurso de que a migração depende.
func declareDatabase(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	if args.Database.ExternalURL != nil {
		return declareExternalDatabase(ctx, names, args, parent)
	}
	return declareNamespacePostgres(ctx, names, args, parent)
}

// declareExternalDatabase guarda a URL num Secret. O provider Kubernetes já
// trata stringData de Secret como segredo no estado.
func declareExternalDatabase(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	return corev1.NewSecret(ctx, names.dbSecret, &corev1.SecretArgs{
		Metadata:   namespaced(names.dbSecret, args.Namespace),
		StringData: pulumi.StringMap{dbSecretKey: args.Database.ExternalURL},
	}, parent)
}

// declareNamespacePostgres sobe um Postgres de uma instância dentro do
// namespace, sem nada no nível do cluster. A senha é gerada e fica só no
// estado do Pulumi e no Secret.
func declareNamespacePostgres(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	password, err := random.NewRandomPassword(ctx, names.database+"-password", &random.RandomPasswordArgs{
		Length: pulumi.Int(passwordLength), Special: pulumi.Bool(false),
	}, parent)
	if err != nil {
		return nil, err
	}
	if err := declarePostgresSecret(ctx, names, args, password.Result, parent); err != nil {
		return nil, err
	}
	labels := pulumi.StringMap{"app.kubernetes.io/name": pulumi.String(names.database)}
	if err := declarePostgresService(ctx, names, args, labels, parent); err != nil {
		return nil, err
	}
	return appsv1.NewStatefulSet(ctx, names.database, postgresStatefulSetArgs(names, args, labels), parent)
}

func declarePostgresSecret(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, password pulumi.StringOutput, parent pulumi.ResourceOption) error {
	uri := pulumi.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		postgresUser, password, names.database, postgresPort, postgresUser)
	_, err := corev1.NewSecret(ctx, names.dbSecret, &corev1.SecretArgs{
		Metadata:   namespaced(names.dbSecret, args.Namespace),
		StringData: pulumi.StringMap{dbPasswordKey: password, dbSecretKey: uri},
	}, parent)
	return err
}

// declarePostgresService é headless: o StatefulSet exige um, e a API acha o
// banco pelo nome dele.
func declarePostgresService(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, labels pulumi.StringMap, parent pulumi.ResourceOption) error {
	_, err := corev1.NewService(ctx, names.database, &corev1.ServiceArgs{
		Metadata: namespaced(names.database, args.Namespace),
		Spec: corev1.ServiceSpecArgs{
			ClusterIP: pulumi.String("None"),
			Selector:  labels,
			Ports:     corev1.ServicePortArray{corev1.ServicePortArgs{Port: pulumi.Int(postgresPort)}},
		},
	}, parent)
	return err
}

func postgresStatefulSetArgs(names resourceNames, args MarkuppArgs, labels pulumi.StringMap) *appsv1.StatefulSetArgs {
	return &appsv1.StatefulSetArgs{
		Metadata: namespaced(names.database, args.Namespace),
		Spec: appsv1.StatefulSetSpecArgs{
			ServiceName: pulumi.String(names.database),
			Replicas:    pulumi.Int(1),
			Selector:    metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: corev1.PodTemplateSpecArgs{
				Metadata: metav1.ObjectMetaArgs{Labels: labels},
				Spec:     corev1.PodSpecArgs{Containers: corev1.ContainerArray{postgresContainer(names)}},
			},
			VolumeClaimTemplates: corev1.PersistentVolumeClaimTypeArray{postgresVolumeClaim(args)},
		},
	}
}

// postgresContainer aponta PGDATA para um subdiretório porque a raiz de um
// volume ext4 tem lost+found, e o initdb recusa diretório não vazio.
func postgresContainer(names resourceNames) corev1.ContainerArgs {
	return corev1.ContainerArgs{
		Name:  pulumi.String("postgres"),
		Image: pulumi.String(postgresImage),
		Env: corev1.EnvVarArray{
			corev1.EnvVarArgs{Name: pulumi.String("POSTGRES_USER"), Value: pulumi.String(postgresUser)},
			corev1.EnvVarArgs{Name: pulumi.String("POSTGRES_DB"), Value: pulumi.String(postgresUser)},
			corev1.EnvVarArgs{Name: pulumi.String("PGDATA"), Value: pulumi.String(postgresDataPath + "/pgdata")},
			secretEnv("POSTGRES_PASSWORD", names.dbSecret, dbPasswordKey),
		},
		Ports:          corev1.ContainerPortArray{corev1.ContainerPortArgs{ContainerPort: pulumi.Int(postgresPort)}},
		VolumeMounts:   corev1.VolumeMountArray{corev1.VolumeMountArgs{Name: pulumi.String("data"), MountPath: pulumi.String(postgresDataPath)}},
		ReadinessProbe: corev1.ProbeArgs{Exec: corev1.ExecActionArgs{Command: pulumi.ToStringArray([]string{"pg_isready", "-U", postgresUser})}},
	}
}

func postgresVolumeClaim(args MarkuppArgs) corev1.PersistentVolumeClaimTypeArgs {
	spec := corev1.PersistentVolumeClaimSpecArgs{
		AccessModes: pulumi.ToStringArray([]string{"ReadWriteOnce"}),
		Resources: corev1.VolumeResourceRequirementsArgs{
			Requests: pulumi.StringMap{"storage": pulumi.String(args.Database.Size)},
		},
	}
	if args.Database.StorageClass != "" {
		spec.StorageClassName = pulumi.String(args.Database.StorageClass)
	}
	return corev1.PersistentVolumeClaimTypeArgs{
		Metadata: metav1.ObjectMetaArgs{Name: pulumi.String("data")},
		Spec:     spec,
	}
}

func databaseURLEnv(names resourceNames) corev1.EnvVarArray {
	return corev1.EnvVarArray{secretEnv("MARKUPP_DATABASE_URL", names.dbSecret, dbSecretKey)}
}

func secretEnv(name, secret, key string) corev1.EnvVarArgs {
	return corev1.EnvVarArgs{
		Name: pulumi.String(name),
		ValueFrom: corev1.EnvVarSourceArgs{SecretKeyRef: corev1.SecretKeySelectorArgs{
			Name: pulumi.String(secret),
			Key:  pulumi.String(key),
		}},
	}
}

func namespaced(name, namespace string) metav1.ObjectMetaArgs {
	return metav1.ObjectMetaArgs{Name: pulumi.String(name), Namespace: pulumi.String(namespace)}
}
