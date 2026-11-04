package main

import (
	"fmt"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	mcpPort = 3000
	// mcpUID é o uid do usuário node da imagem do servidor MCP.
	mcpUID               = 1000
	mcpResourceSecretKey = "resource-secret"
	oauthResourcesKey    = "oauth-resources"
	mcpSecretLength      = 64
)

// MCPArgs liga o servidor MCP (ADR-0045) na mesma origem da API, em /mcp, encaminhado
// pelo Go ao Service dele (ADR-0041).
type MCPArgs struct {
	// Image é a imagem do servidor MCP, com tag de versão.
	Image string `pulumi:"image"`
}

// withoutEmptyMCP trata o bloco de MCP vazio como ausente: o Pulumi YAML não omite
// propriedade, e a stack sem MCP passa o bloco vazio.
func withoutEmptyMCP(args MarkuppArgs) MarkuppArgs {
	if args.MCP != nil && *args.MCP == (MCPArgs{}) {
		args.MCP = nil
	}
	return args
}

// mcpResourceURL é o endereço público do MCP, que é o resource dos tokens dele.
func mcpResourceURL(args MarkuppArgs) string {
	return fmt.Sprintf("https://%s/mcp", args.Host)
}

// mcpProxyEnv diz à API para onde encaminhar /mcp e qual resource aceita, com o segredo
// que só ela e o servidor MCP conhecem.
func mcpProxyEnv(names resourceNames, args MarkuppArgs) corev1.EnvVarArray {
	if args.MCP == nil {
		return nil
	}
	return corev1.EnvVarArray{
		plainEnv("MARKUPP_MCP_UPSTREAM_URL", fmt.Sprintf("http://%s:%d", names.mcp, mcpPort)),
		secretEnv("MARKUPP_OAUTH_RESOURCES", names.mcp, oauthResourcesKey),
	}
}

func declareMCP(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) error {
	if args.MCP == nil {
		return nil
	}
	if err := declareMCPSecret(ctx, names, args, parent); err != nil {
		return err
	}
	// Uma réplica: as sessões do Streamable HTTP moram na memória do processo.
	deployment := singleReplicaDeploymentArgs(names.mcp, args.Namespace, mcpUID, mcpContainer(names, args))
	if _, err := appsv1.NewDeployment(ctx, names.mcp, deployment, parent); err != nil {
		return err
	}
	return declareService(ctx, names.mcp, args.Namespace, mcpPort, parent)
}

// declareMCPSecret guarda o segredo do resource e o par url=segredo que a API lê em
// MARKUPP_OAUTH_RESOURCES.
func declareMCPSecret(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) error {
	secret, err := randomSecret(ctx, names.mcp+"-resource-secret", mcpSecretLength, parent)
	if err != nil {
		return err
	}
	_, err = corev1.NewSecret(ctx, names.mcp, &corev1.SecretArgs{
		Metadata: namespaced(names.mcp, args.Namespace),
		StringData: pulumi.StringMap{
			mcpResourceSecretKey: secret,
			oauthResourcesKey:    pulumi.Sprintf("%s=%s", mcpResourceURL(args), secret),
		},
	}, parent)
	return err
}

func mcpContainer(names resourceNames, args MarkuppArgs) corev1.ContainerArgs {
	return corev1.ContainerArgs{
		Name:  pulumi.String("mcp"),
		Image: pulumi.String(args.MCP.Image),
		Env: corev1.EnvVarArray{
			plainEnv("MARKUPP_MCP_API_URL", fmt.Sprintf("http://%s:%d", names.api, apiPort)),
			plainEnv("MARKUPP_MCP_RESOURCE_URL", mcpResourceURL(args)),
			secretEnv("MARKUPP_MCP_RESOURCE_SECRET", names.mcp, mcpResourceSecretKey),
		},
		Ports:           corev1.ContainerPortArray{corev1.ContainerPortArgs{ContainerPort: pulumi.Int(mcpPort)}},
		ReadinessProbe:  corev1.ProbeArgs{TcpSocket: corev1.TCPSocketActionArgs{Port: pulumi.Int(mcpPort)}},
		SecurityContext: corev1.SecurityContextArgs{AllowPrivilegeEscalation: pulumi.Bool(false)},
	}
}
