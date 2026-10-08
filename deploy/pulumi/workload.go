package main

import (
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
	labels := pulumi.StringMap{"app.kubernetes.io/name": pulumi.String(names.api)}
	if _, err := appsv1.NewDeployment(ctx, names.api, apiDeploymentArgs(names, args, labels), opts...); err != nil {
		return err
	}
	_, err := corev1.NewService(ctx, names.api, &corev1.ServiceArgs{
		Metadata: namespaced(names.api, args.Namespace),
		Spec: corev1.ServiceSpecArgs{
			Selector: labels,
			Ports:    corev1.ServicePortArray{corev1.ServicePortArgs{Port: pulumi.Int(apiPort)}},
		},
	}, opts...)
	return err
}

func apiDeploymentArgs(names resourceNames, args MarkuppArgs, labels pulumi.StringMap) *appsv1.DeploymentArgs {
	container := serverContainer(args, names)
	container.Ports = corev1.ContainerPortArray{corev1.ContainerPortArgs{ContainerPort: pulumi.Int(apiPort)}}
	container.ReadinessProbe = healthzProbe()
	container.LivenessProbe = processProbe()
	return &appsv1.DeploymentArgs{
		Metadata: namespaced(names.api, args.Namespace),
		Spec: appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(*args.APIReplicas),
			Selector: metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: corev1.PodTemplateSpecArgs{
				Metadata: metav1.ObjectMetaArgs{Labels: labels},
				Spec:     corev1.PodSpecArgs{SecurityContext: nonRootPod(), Containers: corev1.ContainerArray{container}},
			},
		},
	}
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
		Path: pulumi.String("/healthz"),
		Port: pulumi.Int(apiPort),
	}}
}

// nonRootPod usa o uid numérico do usuário da imagem: com nome de usuário o
// kubelet não consegue provar que o contêiner não roda como root.
func nonRootPod() corev1.PodSecurityContextArgs {
	return corev1.PodSecurityContextArgs{RunAsNonRoot: pulumi.Bool(true), RunAsUser: pulumi.Int(imageUID)}
}
