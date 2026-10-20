// Package oauth reúne as peças do authorization server OAuth 2.1 (ADR-0039).
package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// Limites do code_verifier na RFC 7636, seção 4.1.
const (
	minVerifierLen = 43
	maxVerifierLen = 128
)

// VerifyPKCE confere o code_verifier contra o code_challenge pelo método S256, o único
// aceito. Verifier fora do formato da RFC 7636 recusa mesmo quando o hash bate.
func VerifyPKCE(verifier, challenge string) bool {
	if !validVerifier(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

// ValidChallenge diz se o code_challenge tem a forma de um SHA256 em base64url sem
// padding: 43 caracteres que decodificam, no modo estrito, em 32 bytes.
func ValidChallenge(challenge string) bool {
	if len(challenge) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(challenge)
	return err == nil && len(decoded) == sha256.Size
}

// validVerifier aplica o tamanho e o conjunto [A-Za-z0-9-._~] da RFC 7636.
func validVerifier(verifier string) bool {
	if len(verifier) < minVerifierLen || len(verifier) > maxVerifierLen {
		return false
	}
	for _, c := range []byte(verifier) {
		if !unreservedChar(c) {
			return false
		}
	}
	return true
}

func unreservedChar(c byte) bool {
	switch {
	case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9':
		return true
	}
	return c == '-' || c == '.' || c == '_' || c == '~'
}
