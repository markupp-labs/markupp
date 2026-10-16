package main

import (
	"fmt"
	"net/url"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
)

const (
	// dbSecretKey é a chave com a URL de conexão que a API e a migração leem.
	dbSecretKey   = "uri"
	dbPasswordKey = "password"
	authSecretKey = "auth-secret"
	// scalerURIKey é a URL do papel que só conta os cofres pendentes, lida pelo
	// TriggerAuthentication do KEDA, e scalerPasswordKey a senha que a migração dá a ele.
	scalerURIKey      = "scaler-uri"
	scalerPasswordKey = "scaler-password"
	scalerUser        = "markupp_scaler"
	postgresImage     = "postgres:17.9-alpine3.23"
	postgresPort      = 5432
	postgresUser      = "markupp"
	passwordLength    = 32
	authSecretLength  = 64
	postgresDataPath  = "/var/lib/postgresql/data"
)

// generatedSecrets são os valores aleatórios do Secret do banco.
type generatedSecrets struct{ auth, scaler pulumi.StringOutput }

// declareDatabase cria o Postgres do namespace ou o Secret do banco externo,
// e devolve o recurso de que a migração depende.
func declareDatabase(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	secrets, err := generateSecrets(ctx, names, parent)
	if err != nil {
		return nil, err
	}
	if args.Database.ExternalURL != nil {
		return declareExternalDatabase(ctx, names, args, secrets, parent)
	}
	return declareNamespacePostgres(ctx, names, args, secrets, parent)
}

func generateSecrets(ctx *pulumi.Context, names resourceNames, parent pulumi.ResourceOption) (generatedSecrets, error) {
	auth, err := randomSecret(ctx, names.component+"-auth-secret", authSecretLength, parent)
	if err != nil {
		return generatedSecrets{}, err
	}
	scaler, err := randomSecret(ctx, names.component+"-scaler-password", passwordLength, parent)
	return generatedSecrets{auth: auth, scaler: scaler}, err
}

func randomSecret(ctx *pulumi.Context, name string, length int, parent pulumi.ResourceOption) (pulumi.StringOutput, error) {
	password, err := random.NewRandomPassword(ctx, name, &random.RandomPasswordArgs{
		Length: pulumi.Int(length), Special: pulumi.Bool(false),
	}, parent)
	if err != nil {
		return pulumi.StringOutput{}, err
	}
	return password.Result, nil
}

// declareExternalDatabase guarda a URL num Secret. O provider Kubernetes já
// trata stringData de Secret como segredo no estado. O host da URL externa
// precisa resolver também do namespace do KEDA.
func declareExternalDatabase(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, secrets generatedSecrets, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	scalerURI := pulumix.Apply2Err(args.Database.ExternalURL.ToStringOutput(), secrets.scaler, func(raw, password string) (string, error) {
		return withUser(raw, scalerUser, password)
	})
	return corev1.NewSecret(ctx, names.dbSecret, &corev1.SecretArgs{
		Metadata: namespaced(names.dbSecret, args.Namespace),
		StringData: withEmailSecret(pulumi.StringMap{
			dbSecretKey: args.Database.ExternalURL, authSecretKey: secrets.auth,
			scalerPasswordKey: secrets.scaler, scalerURIKey: pulumix.Cast[pulumi.StringOutput](scalerURI),
		}, args),
	}, parent)
}

// withUser troca usuário e senha de rawURL, mantendo host, banco e parâmetros.
func withUser(rawURL, user, password string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("externalUrl invalida, esperado postgres://usuario:senha@host:5432/banco: %w", err)
	}
	parsed.User = url.UserPassword(user, password)
	return parsed.String(), nil
}

// declareNamespacePostgres sobe um Postgres de uma instância dentro do
// namespace, sem nada no nível do cluster. A senha é gerada e fica só no
// estado do Pulumi e no Secret.
func declareNamespacePostgres(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, secrets generatedSecrets, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	password, err := randomSecret(ctx, names.database+"-password", passwordLength, parent)
	if err != nil {
		return nil, err
	}
	if err := declarePostgresSecret(ctx, names, args, password, secrets, parent); err != nil {
		return nil, err
	}
	labels := pulumi.StringMap{"app.kubernetes.io/name": pulumi.String(names.database)}
	if err := declarePostgresService(ctx, names, args, labels, parent); err != nil {
		return nil, err
	}
	return appsv1.NewStatefulSet(ctx, names.database, postgresStatefulSetArgs(names, args, labels), parent)
}

// declarePostgresSecret usa o host com namespace: o scaler do KEDA consulta o
// banco a partir do namespace dele.
func declarePostgresSecret(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, password pulumi.StringOutput, secrets generatedSecrets, parent pulumi.ResourceOption) error {
	address := fmt.Sprintf("%s.%s.svc:%d/%s?sslmode=disable", names.database, args.Namespace, postgresPort, postgresUser)
	_, err := corev1.NewSecret(ctx, names.dbSecret, &corev1.SecretArgs{
		Metadata: namespaced(names.dbSecret, args.Namespace),
		StringData: withEmailSecret(pulumi.StringMap{
			dbPasswordKey: password, authSecretKey: secrets.auth, scalerPasswordKey: secrets.scaler,
			dbSecretKey:  pulumi.Sprintf("postgres://%s:%s@%s", postgresUser, password, address),
			scalerURIKey: pulumi.Sprintf("postgres://%s:%s@%s", scalerUser, secrets.scaler, address),
		}, args),
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
	return corev1.EnvVarArray{
		secretEnv("MARKUPP_DATABASE_URL", names.dbSecret, dbSecretKey),
		secretEnv("MARKUPP_AUTH_SECRET", names.dbSecret, authSecretKey),
	}
}

// migrationEnv acrescenta a senha que a migração dá ao papel do scaler.
func migrationEnv(names resourceNames) corev1.EnvVarArray {
	return append(databaseURLEnv(names), secretEnv("MARKUPP_SCALER_PASSWORD", names.dbSecret, scalerPasswordKey))
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
