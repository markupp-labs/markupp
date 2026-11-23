// Package instance reúne o que vale para a instalação inteira, acima dos tenants: quem
// administra a instância e a escolha do provedor de embedding (ADR-0057).
package instance

import "strings"

// Admins são os emails, em minúsculas, das contas que administram a instância, como
// quem hospeda os listou.
type Admins []string

// Includes diz se a conta de email administra a instância: o email precisa estar na
// lista, sem diferença de caixa, e já verificado.
func (a Admins) Includes(email string, verified bool) bool {
	if !verified {
		return false
	}
	for _, admin := range a {
		if strings.EqualFold(admin, email) {
			return true
		}
	}
	return false
}
