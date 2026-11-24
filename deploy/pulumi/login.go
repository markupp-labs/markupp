package main

import (
	"fmt"
	"strings"

	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// LoginProviderArgs é um provedor externo de login (ADR-0054), que a API lê de
// MARKUPP_LOGIN_<ID>_*. O redirect URI a cadastrar no provedor é
// https://<panel.host>/auth/<id>/callback.
type LoginProviderArgs struct {
	// ID identifica o provedor nas variáveis e no callback: letras minúsculas, dígitos e hífen.
	ID string `pulumi:"id"`
	// Name é o nome do botão. Padrão: o id.
	Name string `pulumi:"name,optional"`
	// Issuer é o issuer OpenID Connect. Vazio só no id github.
	Issuer string `pulumi:"issuer,optional"`
	// ClientID e ClientSecret vêm do cadastro do app no provedor.
	ClientID     string             `pulumi:"clientId"`
	ClientSecret pulumi.StringInput `pulumi:"clientSecret" provider:"secret"`
}

// validateLoginProviders recusa o provedor sem id ou sem as credenciais do app; o resto
// a API confere ao subir.
func validateLoginProviders(args MarkuppArgs) error {
	for _, p := range args.LoginProviders {
		if p.ID == "" || p.ClientID == "" || p.ClientSecret == nil {
			return fmt.Errorf("campo loginProviders com id=%q e clientId=%q, esperados id, clientId e clientSecret preenchidos", p.ID, p.ClientID)
		}
	}
	return nil
}

// declareLoginSecret guarda os segredos dos clientes num Secret próprio, criado só com
// provedor: no Secret do banco, uma chave nova o substituiria.
func declareLoginSecret(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) error {
	if len(args.LoginProviders) == 0 {
		return nil
	}
	data := pulumi.StringMap{}
	for _, p := range args.LoginProviders {
		data[loginSecretKey(p.ID)] = p.ClientSecret
	}
	_, err := corev1.NewSecret(ctx, names.login, &corev1.SecretArgs{
		Metadata:   namespaced(names.login, args.Namespace),
		StringData: data,
	}, parent)
	return err
}

func loginSecretKey(id string) string {
	return id + "-client-secret"
}

// loginEnv dá à API a lista de provedores e as variáveis de cada um.
func loginEnv(names resourceNames, args MarkuppArgs) corev1.EnvVarArray {
	if len(args.LoginProviders) == 0 {
		return nil
	}
	ids := make([]string, 0, len(args.LoginProviders))
	env := corev1.EnvVarArray{}
	for _, p := range args.LoginProviders {
		ids = append(ids, p.ID)
		env = append(env, loginProviderEnv(names, p)...)
	}
	return append(corev1.EnvVarArray{plainEnv("MARKUPP_LOGIN_PROVIDERS", strings.Join(ids, ","))}, env...)
}

func loginProviderEnv(names resourceNames, p LoginProviderArgs) corev1.EnvVarArray {
	prefix := "MARKUPP_LOGIN_" + strings.ToUpper(strings.ReplaceAll(p.ID, "-", "_")) + "_"
	env := corev1.EnvVarArray{
		plainEnv(prefix+"CLIENT_ID", p.ClientID),
		secretEnv(prefix+"CLIENT_SECRET", names.login, loginSecretKey(p.ID)),
	}
	if p.Name != "" {
		env = append(env, plainEnv(prefix+"NAME", p.Name))
	}
	if p.Issuer != "" {
		env = append(env, plainEnv(prefix+"ISSUER", p.Issuer))
	}
	return env
}
