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
)

var (
	// ErrInvalidToken indica que a assinatura ou o formato do JWT e invalido.
	ErrInvalidToken = errors.New("token invalido")
	// ErrExpiredToken indica que a validade temporal do JWT expirou.
	ErrExpiredToken = errors.New("token expirado")
)

// Claims são as declarações de identidade e permissão contidas no JWT.
type Claims struct {
	Subject   string `json:"sub"`
	TenantID  string `json:"tenant_id"`
	Role      string `json:"role"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
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

// GenerateTokenPair gera o JWT de acesso e um refresh token aleatório.
func (c *TokenCodec) GenerateTokenPair(userID, tenantID, role string) (TokenPair, error) {
	now := c.clock()
	claims := Claims{
		Subject:   userID,
		TenantID:  tenantID,
		Role:      role,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(c.expiration).Unix(),
	}
	access, err := c.encodeClaims(claims)
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

// ValidateToken valida a assinatura e expiração do JWT, retornando suas claims.
func (c *TokenCodec) ValidateToken(tokenString string) (Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
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

// HashRefreshToken calcula o hash SHA-256 do refresh token para armazenamento seguro.
func HashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
