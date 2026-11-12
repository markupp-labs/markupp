package main

import (
	"fmt"

	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
)

// validateRegistration recusa o cadastro aberto sem email: sem SMTP nada prova o email
// da conta, e qualquer pessoa ocupa o email de outra antes da dona.
func validateRegistration(args MarkuppArgs) error {
	if !args.RegistrationEnabled || args.Email != nil {
		return nil
	}
	return fmt.Errorf("campo registrationEnabled=true sem o bloco email, esperado email.smtpUrl e email.from para verificar o email das contas novas")
}

// registrationEnv abre o cadastro na API quando pedido.
func registrationEnv(args MarkuppArgs) corev1.EnvVarArray {
	if !args.RegistrationEnabled {
		return nil
	}
	return corev1.EnvVarArray{plainEnv("MARKUPP_AUTH_REGISTRATION_ENABLED", "true")}
}
