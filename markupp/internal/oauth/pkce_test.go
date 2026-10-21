package oauth_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// Vetor de teste do apêndice B da RFC 7636.
const (
	verifierDaRFC  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challengeDaRFC = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
)

func TestVerifyPKCE_VetorDaRFC_Aceita(t *testing.T) {
	assert.True(t, oauth.VerifyPKCE(verifierDaRFC, challengeDaRFC))
}

func TestVerifyPKCE_EntradaInvalida_Recusa(t *testing.T) {
	casos := map[string]struct{ verifier, challenge string }{
		"verifier de outro challenge": {"a" + verifierDaRFC[1:], challengeDaRFC},
		"challenge alterado":          {verifierDaRFC, "F" + challengeDaRFC[1:]},
		"challenge vazio":             {verifierDaRFC, ""},
		"verifier vazio":              {"", challengeDaRFC},
		"verifier igual ao challenge": {challengeDaRFC, challengeDaRFC},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			assert.False(t, oauth.VerifyPKCE(caso.verifier, caso.challenge))
		})
	}
}

func TestVerifyPKCE_VerifierForaDaRFC_RecusaMesmoComHashCerto(t *testing.T) {
	// Cada challenge é o SHA256 certo do verifier: a recusa vem só da RFC 7636.
	casos := map[string]struct{ verifier, challenge string }{
		"42 caracteres":     {strings.Repeat("a", 42), "elOGB_2quSlplZKfRRVlu7gULhhEEXMiqv0rPXawGv8"},
		"129 caracteres":    {strings.Repeat("a", 129), "wSywJKLlVRzKDgj86PHF4xRVXMP-9jKe6ZSj23UhZq4"},
		"caractere de fora": {strings.Repeat("a", 42) + "+", "iwXbWFm6ct1JDeJlZO8FYEXe0UbbNRVyu6etiydm5O8"},
		"espaco":            {strings.Repeat("a", 42) + " ", "yRMT0C56DfjhDGsdwIPX1T1Wi1Ee-Gr8X8_kFJSKYLw"},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			assert.False(t, oauth.VerifyPKCE(caso.verifier, caso.challenge))
		})
	}
}

func TestVerifyPKCE_VerifierNosLimitesDoTamanho_Aceita(t *testing.T) {
	// Challenges calculados com SHA256 e base64url sem padding.
	casos := map[string]struct{ verifier, challenge string }{
		"43 caracteres":      {strings.Repeat("a", 43), "ZtNPunH49FD35FWYhT5Tv8I7vRKQJ8uxMaL0_9eHjNA"},
		"128 caracteres":     {strings.Repeat("~", 128), "zNhOm5Jyonenca7bQzzpjUpwFDVrfhrbbOGCqgWA6HU"},
		"conjunto permitido": {"AZaz09-._~" + strings.Repeat("x", 33), "V88YhcQsQT6uTW_CUZOYcWMxrYSbJMac2hG3lcnHRuU"},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			assert.True(t, oauth.VerifyPKCE(caso.verifier, caso.challenge))
		})
	}
}

func TestValidChallenge_ChallengeDaRFC_Aceita(t *testing.T) {
	assert.True(t, oauth.ValidChallenge(challengeDaRFC))
}

func TestValidChallenge_ForaDoFormato_Recusa(t *testing.T) {
	casos := map[string]string{
		"vazio":                   "",
		"42 caracteres":           challengeDaRFC[:42],
		"44 caracteres":           challengeDaRFC + "A",
		"com padding":             challengeDaRFC[:42] + "=",
		"alfabeto base64 padrao":  "+" + challengeDaRFC[1:],
		"barra do base64 padrao":  "/" + challengeDaRFC[1:],
		"bits finais nao zerados": challengeDaRFC[:42] + "N",
	}
	for nome, challenge := range casos {
		t.Run(nome, func(t *testing.T) {
			assert.False(t, oauth.ValidChallenge(challenge))
		})
	}
}
