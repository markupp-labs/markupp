package main

import (
	"fmt"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// panelPort é a porta do Thruster na frente do Puma: a 80 da imagem exige
	// root para abrir, e o painel roda sem root.
	panelPort = 8080
	// panelUID é o uid do usuário rails criado no Dockerfile do painel.
	panelUID            = 1000
	panelListener       = "https-panel"
	secretKeyBaseKey    = "secret-key-base"
	secretKeyBaseLength = 128
	panelRelaySecretKey = "relay-secret"
)

// PanelArgs liga o painel web (ADR-0031), num host próprio: no host da API as
// rotas dos dois colidem.
type PanelArgs struct {
	// Image é a imagem do painel, com tag de versão.
	Image string `pulumi:"image"`
	// Host é o domínio público do painel.
	Host string `pulumi:"host"`
	// SectionName é o listener do Gateway existente para o host do painel. Vazio
	// deixa o Gateway escolher pelo host.
	SectionName string `pulumi:"sectionName,optional"`
}

// withoutEmptyPanel trata o bloco de painel vazio como ausente: o Pulumi YAML não
// omite propriedade, e a stack sem painel passa o bloco vazio.
func withoutEmptyPanel(args MarkuppArgs) MarkuppArgs {
	if args.Panel != nil && *args.Panel == (PanelArgs{}) {
		args.Panel = nil
	}
	return args
}

func validatePanel(args MarkuppArgs) error {
	if args.Panel == nil {
		return nil
	}
	if args.Panel.Image == "" || args.Panel.Host == "" {
		return fmt.Errorf("campo panel.image ou panel.host vazio (panel.image=%q panel.host=%q), esperado imagem com tag e domínio do painel",
			args.Panel.Image, args.Panel.Host)
	}
	return nil
}

// panelURL é o endereço público do painel, vazio sem painel.
func panelURL(args MarkuppArgs) string {
	if args.Panel == nil {
		return ""
	}
	return "https://" + args.Panel.Host
}

// panelAPIEnv diz à API para onde mandar o consentimento do OAuth (ADR-0047) e o segredo
// com que o painel se apresenta como relay da leitura pública (ADR-0050).
func panelAPIEnv(names resourceNames, args MarkuppArgs) corev1.EnvVarArray {
	if args.Panel == nil {
		return nil
	}
	return corev1.EnvVarArray{plainEnv("MARKUPP_PANEL_URL", panelURL(args)), panelRelaySecretEnv(names)}
}

// panelRelaySecretEnv lê o segredo de relay do painel, que só a API e o painel conhecem:
// com ele o limite por IP da leitura pública conta o visitante, e não o painel.
func panelRelaySecretEnv(names resourceNames) corev1.EnvVarInput {
	return secretEnv("MARKUPP_PANEL_RELAY_SECRET", names.panel, panelRelaySecretKey)
}

// panelMCPEnv dá ao painel o endereço do MCP só quando a API o encaminha: sem
// ele, a página Agents avisa que o MCP está desligado em vez de anunciar um /mcp
// que daria 404.
func panelMCPEnv(args MarkuppArgs) corev1.EnvVarArray {
	if args.MCP == nil {
		return nil
	}
	return corev1.EnvVarArray{plainEnv("MARKUPP_MCP_URL", mcpResourceURL(args))}
}

func declarePanel(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) error {
	if args.Panel == nil {
		return nil
	}
	if err := declarePanelSecret(ctx, names, args, parent); err != nil {
		return err
	}
	// Uma réplica: a renovação do token mora na memória do processo, e mais réplicas
	// pedem afinidade por cookie no Gateway (web/config/puma.rb).
	deployment := singleReplicaDeploymentArgs(names.panel, args.Namespace, panelUID, panelContainer(names, args))
	if _, err := appsv1.NewDeployment(ctx, names.panel, deployment, parent); err != nil {
		return err
	}
	return declareService(ctx, names.panel, args.Namespace, corev1.ServicePortArgs{Port: pulumi.Int(panelPort)}, parent)
}

// declarePanelSecret guarda a secret_key_base, que cifra o cookie de sessão, e o segredo
// de relay do painel.
func declarePanelSecret(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) error {
	keyBase, err := randomSecret(ctx, names.panel+"-secret-key-base", secretKeyBaseLength, parent)
	if err != nil {
		return err
	}
	relay, err := randomSecret(ctx, names.panel+"-relay-secret", mcpSecretLength, parent)
	if err != nil {
		return err
	}
	_, err = corev1.NewSecret(ctx, names.panel, &corev1.SecretArgs{
		Metadata:   namespaced(names.panel, args.Namespace),
		StringData: pulumi.StringMap{secretKeyBaseKey: keyBase, panelRelaySecretKey: relay},
	}, parent)
	return err
}

// panelContainer fala com a API pelo Service, do lado do servidor: o navegador
// nunca chama a API. As URLs públicas da API e do MCP vão só para a página Agents,
// que mostra aos clientes MCP o endereço do MCP da instância.
func panelContainer(names resourceNames, args MarkuppArgs) corev1.ContainerArgs {
	env := corev1.EnvVarArray{
		plainEnv("MARKUPP_API_URL", fmt.Sprintf("http://%s:%d", names.api, apiServicePort)),
		publicURLEnv(args),
		secretEnv("SECRET_KEY_BASE", names.panel, secretKeyBaseKey),
		plainEnv("HTTP_PORT", fmt.Sprint(panelPort)),
		panelRelaySecretEnv(names),
	}
	return corev1.ContainerArgs{
		Name:  pulumi.String("panel"),
		Image: pulumi.String(args.Panel.Image),
		Env:   append(env, panelMCPEnv(args)...),
		Ports: corev1.ContainerPortArray{corev1.ContainerPortArgs{ContainerPort: pulumi.Int(panelPort)}},
		ReadinessProbe: corev1.ProbeArgs{HttpGet: corev1.HTTPGetActionArgs{
			Path: pulumi.String("/up"), Port: pulumi.Int(panelPort),
		}},
		LivenessProbe:   corev1.ProbeArgs{TcpSocket: corev1.TCPSocketActionArgs{Port: pulumi.Int(panelPort)}},
		SecurityContext: corev1.SecurityContextArgs{AllowPrivilegeEscalation: pulumi.Bool(false)},
	}
}
