package main

import (
	"strconv"
	"strings"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	batchv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/batch/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// migrationBackoff cobre o tempo de o PostgreSQL aceitar conexão na primeira
// instalação: o Job falha rápido e tenta de novo até o banco responder.
const migrationBackoff = 10

// imageUID é o uid do usuário markupp criado no Dockerfile do servidor.
const imageUID = 10001

// declareMigrationJob aplica o schema antes da API. Sem nome fixo, o Pulumi
// gera um nome por versão e troca o Job quando a imagem muda.
func declareMigrationJob(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, db pulumi.Resource, parent pulumi.ResourceOption) (pulumi.Resource, error) {
	container := serverContainer(args, names)
	container.Args = pulumi.StringArray{pulumi.String("migrate")}
	container.Env = migrationEnv(names)
	return batchv1.NewJob(ctx, names.component+"-migrate", &batchv1.JobArgs{
		Metadata: metav1.ObjectMetaArgs{Namespace: pulumi.String(args.Namespace)},
		Spec: batchv1.JobSpecArgs{
			BackoffLimit: pulumi.Int(migrationBackoff),
			Template: corev1.PodTemplateSpecArgs{Spec: corev1.PodSpecArgs{
				RestartPolicy:   pulumi.String("Never"),
				SecurityContext: nonRootPod(),
				Containers:      corev1.ContainerArray{container},
			}},
		},
	}, parent, pulumi.DependsOn([]pulumi.Resource{db}))
}

func declareAPI(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, opts ...pulumi.ResourceOption) error {
	if _, err := appsv1.NewDeployment(ctx, names.api, apiDeploymentArgs(names, args), opts...); err != nil {
		return err
	}
	return declareService(ctx, names.api, args.Namespace, apiPort, opts...)
}

// declareService expõe na porta os pods do Deployment de mesmo nome.
func declareService(ctx *pulumi.Context, name, namespace string, port int, opts ...pulumi.ResourceOption) error {
	_, err := corev1.NewService(ctx, name, &corev1.ServiceArgs{
		Metadata: namespaced(name, namespace),
		Spec: corev1.ServiceSpecArgs{
			Selector: appLabels(name),
			Ports:    corev1.ServicePortArray{corev1.ServicePortArgs{Port: pulumi.Int(port)}},
		},
	}, opts...)
	return err
}

// singleReplicaDeploymentArgs é o Deployment de uma réplica, sem root, dos processos
// que guardam estado na memória: o painel e o servidor MCP.
func singleReplicaDeploymentArgs(name, namespace string, uid int, container corev1.ContainerArgs) *appsv1.DeploymentArgs {
	return &appsv1.DeploymentArgs{
		Metadata: namespaced(name, namespace),
		Spec: appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Selector: metav1.LabelSelectorArgs{MatchLabels: appLabels(name)},
			Template: corev1.PodTemplateSpecArgs{
				Metadata: metav1.ObjectMetaArgs{Labels: appLabels(name)},
				Spec: corev1.PodSpecArgs{
					SecurityContext: corev1.PodSecurityContextArgs{RunAsNonRoot: pulumi.Bool(true), RunAsUser: pulumi.Int(uid)},
					Containers:      corev1.ContainerArray{container},
				},
			},
		},
	}
}

func apiDeploymentArgs(names resourceNames, args MarkuppArgs) *appsv1.DeploymentArgs {
	container := serverContainer(args, names)
	env := append(databaseURLEnv(names), inviteByEmailEnv(args)...)
	env = append(env, publicURLEnv(args))
	env = append(env, rateLimitEnv(args)...)
	env = append(env, panelURLEnv(args)...)
	env = append(env, mcpProxyEnv(names, args)...)
	container.Env = env
	container.Ports = corev1.ContainerPortArray{corev1.ContainerPortArgs{ContainerPort: pulumi.Int(apiPort)}}
	container.ReadinessProbe = healthzProbe()
	container.LivenessProbe = processProbe()
	return serverDeploymentArgs(names.api, args, pulumi.Int(*args.APIReplicas), container)
}

// rateLimitEnv repassa à API só as opções do limite de tentativas que foram informadas.
func rateLimitEnv(args MarkuppArgs) corev1.EnvVarArray {
	var env corev1.EnvVarArray
	if len(args.TrustedProxies) > 0 {
		env = append(env, plainEnv("MARKUPP_TRUSTED_PROXIES", strings.Join(args.TrustedProxies, ",")))
	}
	if args.RateLimitPerMinute != nil {
		env = append(env, plainEnv("MARKUPP_RATE_LIMIT_PER_MINUTE", strconv.Itoa(*args.RateLimitPerMinute)))
	}
	return env
}

func plainEnv(name, value string) corev1.EnvVarArgs {
	return corev1.EnvVarArgs{Name: pulumi.String(name), Value: pulumi.String(value)}
}

// publicURLEnv dá à API o issuer do OAuth: o host que o Gateway publica com TLS. Sem
// ela, o servidor anunciaria localhost (ADR-0041).
func publicURLEnv(args MarkuppArgs) corev1.EnvVarArgs {
	return corev1.EnvVarArgs{Name: pulumi.String("MARKUPP_PUBLIC_URL"), Value: pulumi.Sprintf("https://%s", args.Host)}
}

// serverDeploymentArgs é o Deployment de um contêiner só do servidor, sem root. Sem
// replicas, quem decide o número de réplicas é o KEDA.
func serverDeploymentArgs(name string, args MarkuppArgs, replicas pulumi.IntPtrInput, container corev1.ContainerArgs) *appsv1.DeploymentArgs {
	return &appsv1.DeploymentArgs{
		Metadata: namespaced(name, args.Namespace),
		Spec: appsv1.DeploymentSpecArgs{
			Replicas: replicas,
			Selector: metav1.LabelSelectorArgs{MatchLabels: appLabels(name)},
			Template: corev1.PodTemplateSpecArgs{
				Metadata: metav1.ObjectMetaArgs{Labels: appLabels(name)},
				Spec:     corev1.PodSpecArgs{SecurityContext: nonRootPod(), Containers: corev1.ContainerArray{container}},
			},
		},
	}
}

func appLabels(name string) pulumi.StringMap {
	return pulumi.StringMap{"app.kubernetes.io/name": pulumi.String(name)}
}

func serverContainer(args MarkuppArgs, names resourceNames) corev1.ContainerArgs {
	return corev1.ContainerArgs{
		Name:  pulumi.String("markupp"),
		Image: pulumi.String(args.Image),
		Env:   databaseURLEnv(names),
		SecurityContext: corev1.SecurityContextArgs{
			AllowPrivilegeEscalation: pulumi.Bool(false),
			ReadOnlyRootFilesystem:   pulumi.Bool(true),
		},
	}
}

// processProbe só confere se o processo aceita conexão. Ela não passa pelo
// banco: com o /healthz, uma queda do PostgreSQL reiniciaria todas as réplicas
// da API, e a readiness já tira a réplica do tráfego nesse caso.
func processProbe() corev1.ProbeArgs {
	return corev1.ProbeArgs{TcpSocket: corev1.TCPSocketActionArgs{Port: pulumi.Int(apiPort)}}
}

func healthzProbe() corev1.ProbeArgs {
	return corev1.ProbeArgs{HttpGet: corev1.HTTPGetActionArgs{
		Path: pulumi.String("/v1/healthz"),
		Port: pulumi.Int(apiPort),
	}}
}

// nonRootPod usa o uid numérico do usuário da imagem: com nome de usuário o
// kubelet não consegue provar que o contêiner não roda como root.
func nonRootPod() corev1.PodSecurityContextArgs {
	return corev1.PodSecurityContextArgs{RunAsNonRoot: pulumi.Bool(true), RunAsUser: pulumi.Int(imageUID)}
}
