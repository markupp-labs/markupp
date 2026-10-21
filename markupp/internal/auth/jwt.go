// Package auth concentra as regras de negócio de autenticação, contas,
// controle de acesso e emissão de tokens.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

var (
	// ErrInvalidToken indica que a assinatura ou o formato do JWT e invalido.
	ErrInvalidToken = errors.New("token invalido")
	// ErrExpiredToken indica que a validade temporal do JWT expirou.
	ErrExpiredToken = errors.New("token expirado")
)

type tokenHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// Claims são as declarações de identidade do JWT. Tenant e papel ficam fora: vêm do
// vínculo no banco a cada requisição, e nunca envelhecem dentro do token. Scope só
// existe no token que a troca OAuth entrega; vazio é token de login, de acesso inteiro.
type Claims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Scope     string `json:"scope,omitempty"`
}

// TokenPair representa o par de tokens emitidos para a sessão.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// TokenCodec assina e decodifica tokens JWT stateless via HMAC-SHA256.
type TokenCodec struct {
	secret     []byte
	expiration time.Duration
	clock      func() time.Time
}

// NewTokenCodec cria o codec sobre o segredo, validade e relogio fornecidos.
func NewTokenCodec(secret string, expiration time.Duration, clock func() time.Time) *TokenCodec {
	return &TokenCodec{
		secret:     []byte(secret),
		expiration: expiration,
		clock:      clock,
	}
}

func (c *TokenCodec) newAccessClaims(userID string) Claims {
	now := c.clock()
	return Claims{
		Subject:   userID,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(c.expiration).Unix(),
	}
}

// GenerateTokenPair gera o JWT de acesso e um refresh token aleatório.
func (c *TokenCodec) GenerateTokenPair(userID string) (TokenPair, error) {
	access, err := c.encodeClaims(c.newAccessClaims(userID))
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := generateRandomHex(32)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(c.expiration.Seconds()),
	}, nil
}

func decodeHeader(headerPart string) error {
	raw, err := base64.RawURLEncoding.DecodeString(headerPart)
	if err != nil {
		return ErrInvalidToken
	}
	var h tokenHeader
	if err := json.Unmarshal(raw, &h); err != nil {
		return ErrInvalidToken
	}
	if h.Alg != "HS256" || h.Typ != "JWT" {
		return ErrInvalidToken
	}
	return nil
}

// ValidateToken valida a assinatura e expiração do JWT, retornando suas claims.
func (c *TokenCodec) ValidateToken(tokenString string) (Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	if err := decodeHeader(parts[0]); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if !c.verifySignature(parts[0], parts[1], parts[2]) {
		return Claims{}, ErrInvalidToken
	}
	claims, err := decodeClaims(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.ExpiresAt < c.clock().Unix() {
		return Claims{}, ErrExpiredToken
	}
	return claims, nil
}

func (c *TokenCodec) encodeClaims(claims Claims) (string, error) {
	headerJSON := `{"alg":"HS256","typ":"JWT"}`
	headerPart := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("serializar claims: %w", err)
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := c.sign(headerPart + "." + payloadPart)
	return headerPart + "." + payloadPart + "." + signature, nil
}

func (c *TokenCodec) sign(message string) string {
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (c *TokenCodec) verifySignature(headerPart, payloadPart, sigPart string) bool {
	expectedSig := c.sign(headerPart + "." + payloadPart)
	return hmac.Equal([]byte(expectedSig), []byte(sigPart))
}

func decodeClaims(payloadPart string) (Claims, error) {
	raw, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return Claims{}, err
	}
	var claims Claims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return Claims{}, err
	}
	return claims, nil
}

func generateRandomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("gerar bytes aleatorios: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// NewOpaqueToken gera um token opaco de 32 bytes aleatórios, em hexadecimal, como o
// de convite. Só o hash dele vai para o banco.
func NewOpaqueToken() (string, error) {
	return generateRandomHex(32)
}

// HashOpaqueToken calcula o hash SHA-256 de um token opaco (refresh ou convite), que
// é o que o banco guarda no lugar do token.
func HashOpaqueToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// IssueScopedToken emite o token da API com escopo que a troca de token OAuth entrega:
// o mesmo JWT do login, com validade ttl e os escopos separados por espaço na claim scope.
func (c *TokenCodec) IssueScopedToken(userID string, scopes []string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", fmt.Errorf("auth: validade %s do token com escopo invalida; esperado maior que zero", ttl)
	}
	if err := checkTokenScopes(scopes); err != nil {
		return "", err
	}
	now := c.clock()
	return c.encodeClaims(Claims{
		Subject: userID, IssuedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix(), Scope: strings.Join(scopes, " "),
	})
}

// checkTokenScopes barra lista vazia e escopo vazio ou com espaço, que fariam a claim
// scope sumir ou mudar de sentido. Sem claim scope o token valeria como de login.
func checkTokenScopes(scopes []string) error {
	if len(scopes) == 0 {
		return errors.New("auth: token com escopo sem nenhum escopo; esperado ao menos um")
	}
	for _, scope := range scopes {
		if scope == "" || strings.ContainsFunc(scope, unicode.IsSpace) {
			return fmt.Errorf("auth: escopo %q invalido; esperado texto sem espacos", scope)
		}
	}
	return nil
}
