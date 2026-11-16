package main

import (
	"strings"

	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
)

// secretKeyKey é a chave do Secret do banco com a MARKUPP_SECRET_KEY, que cifra os
// segredos guardados no banco (ADR-0057): sem ela, uma cópia do banco os revelaria.
const secretKeyKey = "secret-key"

// instanceEnv dá à API a chave secreta e, quando há, quem administra a instância.
func instanceEnv(names resourceNames, args MarkuppArgs) corev1.EnvVarArray {
	env := corev1.EnvVarArray{secretEnv("MARKUPP_SECRET_KEY", names.dbSecret, secretKeyKey)}
	if len(args.InstanceAdmins) == 0 {
		return env
	}
	return append(env, plainEnv("MARKUPP_INSTANCE_ADMINS", strings.Join(args.InstanceAdmins, ",")))
}
