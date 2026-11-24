package main

import (
	"fmt"
	"strings"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// modelDir é onde o embedder guarda os pesos do modelo local (ADR-0056).
	modelDir = "/var/lib/markupp/models"
	// modelVolumeSize cabe os pesos cheios do modelo local, 1,2 GB, com folga.
	modelVolumeSize = "2Gi"
	// embedderSuffix é o sufixo do repositório da imagem com o ONNX Runtime.
	embedderSuffix = "-embedder"
)

// declareEmbedder sobe o indexador de embedding em laço, uma réplica com o volume do
// modelo local. Sem MARKUPP_EMBEDDING_PROVIDER, vale a escolha do painel (ADR-0057).
func declareEmbedder(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, opts pulumi.ResourceOption) error {
	image, err := embedderImageOf(args)
	if err != nil {
		return err
	}
	if usesModelVolume(args) {
		if _, err := corev1.NewPersistentVolumeClaim(ctx, names.embedderModels, modelVolumeClaimArgs(names, args), opts); err != nil {
			return err
		}
	}
	_, err = appsv1.NewDeployment(ctx, names.embedder, embedderDeploymentArgs(names, args, image), opts)
	return err
}

// embedderImageOf é embedderImage ou, sem ele, a imagem do servidor com -embedder no
// repositório e a mesma tag. A imagem por digest não tem tag para repetir.
func embedderImageOf(args MarkuppArgs) (string, error) {
	if args.EmbedderImage != "" {
		return args.EmbedderImage, nil
	}
	if strings.Contains(args.Image, "@") {
		return "", fmt.Errorf("image=%q fixada por digest, esperado embedderImage com a imagem markupp-embedder da mesma versão", args.Image)
	}
	slash := strings.LastIndex(args.Image, "/")
	colon := strings.LastIndex(args.Image, ":")
	if colon <= slash {
		return args.Image + embedderSuffix, nil
	}
	return args.Image[:colon] + embedderSuffix + args.Image[colon:], nil
}

func modelVolumeClaimArgs(names resourceNames, args MarkuppArgs) *corev1.PersistentVolumeClaimArgs {
	spec := corev1.PersistentVolumeClaimSpecArgs{
		AccessModes: pulumi.ToStringArray([]string{"ReadWriteOnce"}),
		Resources:   corev1.VolumeResourceRequirementsArgs{Requests: pulumi.StringMap{"storage": pulumi.String(modelVolumeSize)}},
	}
	if args.Database.StorageClass != "" {
		spec.StorageClassName = pulumi.String(args.Database.StorageClass)
	}
	return &corev1.PersistentVolumeClaimArgs{Metadata: namespaced(names.embedderModels, args.Namespace), Spec: spec}
}

// embedderDeploymentArgs troca no Recreate: o volume RWO não monta em duas réplicas.
// O fsGroup deixa o usuário da imagem escrever no volume, e o /tmp fica gravável.
func embedderDeploymentArgs(names resourceNames, args MarkuppArgs, image string) *appsv1.DeploymentArgs {
	return &appsv1.DeploymentArgs{
		Metadata: namespaced(names.embedder, args.Namespace),
		Spec: appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Strategy: appsv1.DeploymentStrategyArgs{Type: pulumi.String("Recreate")},
			Selector: metav1.LabelSelectorArgs{MatchLabels: appLabels(names.embedder)},
			Template: corev1.PodTemplateSpecArgs{
				Metadata: metav1.ObjectMetaArgs{Labels: appLabels(names.embedder)},
				Spec: corev1.PodSpecArgs{
					EnableServiceLinks: noServiceLinks(),
					SecurityContext:    corev1.PodSecurityContextArgs{RunAsNonRoot: pulumi.Bool(true), RunAsUser: pulumi.Int(imageUID), FsGroup: pulumi.Int(imageUID)},
					Containers:         corev1.ContainerArray{embedderContainer(names, args, image)},
					Volumes: corev1.VolumeArray{
						modelVolume(names, args),
						corev1.VolumeArgs{Name: pulumi.String("tmp"), EmptyDir: corev1.EmptyDirVolumeSourceArgs{}},
					},
				},
			},
		},
	}
}

func usesModelVolume(args MarkuppArgs) bool {
	return args.EmbedderModelVolume == nil || *args.EmbedderModelVolume
}

// modelVolume é o PersistentVolumeClaim dos pesos ou, sem ele, um emptyDir.
func modelVolume(names resourceNames, args MarkuppArgs) corev1.VolumeArgs {
	volume := corev1.VolumeArgs{Name: pulumi.String("models")}
	if !usesModelVolume(args) {
		volume.EmptyDir = corev1.EmptyDirVolumeSourceArgs{}
		return volume
	}
	volume.PersistentVolumeClaim = corev1.PersistentVolumeClaimVolumeSourceArgs{ClaimName: pulumi.String(names.embedderModels)}
	return volume
}

// embedderContainer só passa o banco e a chave que decifra a chave do endpoint remoto.
func embedderContainer(names resourceNames, args MarkuppArgs, image string) corev1.ContainerArgs {
	container := serverContainer(args, names)
	container.Image = pulumi.String(image)
	container.Args = pulumi.StringArray{pulumi.String("embed-loop")}
	container.Env = corev1.EnvVarArray{
		secretEnv("MARKUPP_DATABASE_URL", names.dbSecret, dbSecretKey),
		secretEnv("MARKUPP_SECRET_KEY", names.dbSecret, secretKeyKey),
	}
	container.VolumeMounts = corev1.VolumeMountArray{
		corev1.VolumeMountArgs{Name: pulumi.String("models"), MountPath: pulumi.String(modelDir)},
		corev1.VolumeMountArgs{Name: pulumi.String("tmp"), MountPath: pulumi.String("/tmp")},
	}
	return container
}
