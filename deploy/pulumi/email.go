package main

import (
	"fmt"
	"net/mail"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	smtpURLKey                = "smtp-url"
	pendingNotificationsQuery = "SELECT markupp_pending_notifications()"
)

// EmailArgs liga o envio de email (ADR-0021). Sem ele, o convite devolve o token
// na resposta da API e nenhum notificador sobe.
type EmailArgs struct {
	// SMTPURL é o servidor, smtp://usuario:senha@host:porta ou smtps://.
	SMTPURL pulumi.StringInput `pulumi:"smtpUrl" provider:"secret"`
	// From é o remetente dos emails.
	From string `pulumi:"from"`
	// PanelURL é a URL base do painel web, que põe nos emails o link da página que
	// recebe o token (MARKUPP_PANEL_URL). Vazia, o notificador usa o painel local padrão.
	PanelURL string `pulumi:"panelUrl,optional"`
}

// withEmailSecret acrescenta a URL do SMTP ao Secret do markupp quando há email.
func withEmailSecret(secretValues pulumi.StringMap, args MarkuppArgs) pulumi.StringMap {
	if args.Email != nil {
		secretValues[smtpURLKey] = args.Email.SMTPURL
	}
	return secretValues
}

// validateEmailSender recusa o remetente que o notificador recusaria ao subir.
func validateEmailSender(args MarkuppArgs) error {
	if args.Email == nil {
		return nil
	}
	if _, err := mail.ParseAddress(args.Email.From); err != nil {
		return fmt.Errorf("campo email.from=%q, esperado usuario@dominio ou Nome <usuario@dominio>: %w", args.Email.From, err)
	}
	return nil
}

// inviteByEmailEnv diz à API que o convite sai por email. Ela não recebe o SMTP:
// quem envia é só o notificador.
func inviteByEmailEnv(args MarkuppArgs) corev1.EnvVarArray {
	if args.Email == nil {
		return nil
	}
	return corev1.EnvVarArray{plainEnv("MARKUPP_INVITE_BY_EMAIL", "true")}
}

// smtpEnv dá ao notificador o servidor, o remetente e, quando informada, a URL do painel.
func smtpEnv(names resourceNames, args MarkuppArgs) corev1.EnvVarArray {
	env := corev1.EnvVarArray{
		secretEnv("MARKUPP_SMTP_URL", names.dbSecret, smtpURLKey),
		plainEnv("MARKUPP_SMTP_FROM", args.Email.From),
	}
	if args.Email.PanelURL != "" {
		env = append(env, plainEnv("MARKUPP_PANEL_URL", args.Email.PanelURL))
	}
	return env
}

// declareNotifier cria o Deployment do notify-loop e o ScaledObject que o leva a
// zero réplicas enquanto não há email vencido na fila, e a uma quando há.
func declareNotifier(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, opts pulumi.ResourceOption) error {
	if args.Email == nil {
		return nil
	}
	container := serverContainer(args, names)
	container.Args = pulumi.StringArray{pulumi.String("notify-loop")}
	container.Env = append(databaseURLEnv(names), smtpEnv(names, args)...)
	if _, err := appsv1.NewDeployment(ctx, names.notifier, serverDeploymentArgs(names.notifier, args, nil, container), opts); err != nil {
		return err
	}
	return declareCustom(ctx, "keda.sh/v1alpha1", "ScaledObject", names.notifier, args, pulumi.Map{
		"scaleTargetRef":  pulumi.Map{"name": pulumi.String(names.notifier)},
		"minReplicaCount": pulumi.Int(0),
		"maxReplicaCount": pulumi.Int(1),
		"pollingInterval": pulumi.Int(kedaPollingSeconds),
		"triggers":        pulumi.Array{pendingRowsTrigger(names, pendingNotificationsQuery)},
	}, opts)
}
