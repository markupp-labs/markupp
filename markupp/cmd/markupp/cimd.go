package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/config"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

// cimdLimits são os limites da busca de Client ID Metadata Documents já convertidos
// da config para as unidades do pacote cimd.
type cimdLimits struct {
	Timeout      time.Duration
	MaxBodyBytes int64
	MinTTL       time.Duration
	MaxTTL       time.Duration
}

// cimdLimitsFrom traduz os limites da config para o fetcher.
func cimdLimitsFrom(cfg config.Config) cimdLimits {
	return cimdLimits{
		Timeout:      time.Duration(cfg.OAuthCIMDTimeoutSeconds) * time.Second,
		MaxBodyBytes: int64(cfg.OAuthCIMDMaxKB) << 10,
		MinTTL:       time.Duration(cfg.OAuthCIMDCacheMinMinutes) * time.Minute,
		MaxTTL:       time.Duration(cfg.OAuthCIMDCacheMaxHours) * time.Hour,
	}
}

// cimdClientMetadataFetcher entrega ao serviço OAuth os documentos que o cimd.Fetcher
// busca e valida.
type cimdClientMetadataFetcher struct {
	fetcher *cimd.Fetcher
}

// newCIMDClientMetadataFetcher monta o fetcher com os limites e a política PublicOnly;
// extra vem depois e só existe para o teste trocar autoridades e política de IP.
func newCIMDClientMetadataFetcher(limits cimdLimits, extra ...cimd.Option) (*cimdClientMetadataFetcher, error) {
	opts := append([]cimd.Option{
		cimd.WithTimeout(limits.Timeout),
		cimd.WithMaxBodyBytes(limits.MaxBodyBytes),
		cimd.WithTTLBounds(limits.MinTTL, limits.MaxTTL),
		cimd.WithIPPolicy(cimd.PublicOnly),
	}, extra...)
	fetcher, err := cimd.NewFetcher(opts...)
	if err != nil {
		return nil, fmt.Errorf("montar o fetcher de documentos cimd: %w", err)
	}
	return &cimdClientMetadataFetcher{fetcher: fetcher}, nil
}

// FetchClientMetadata implementa oauth.ClientMetadataFetcher. O erro embrulhado leva
// detalhe de rede e o corpo do documento, então serve só para o log, nunca para o cliente.
func (a *cimdClientMetadataFetcher) FetchClientMetadata(ctx context.Context, clientID string) (oauth.ClientMetadata, error) {
	meta, err := a.fetcher.Fetch(ctx, clientID)
	if err != nil {
		return oauth.ClientMetadata{}, fmt.Errorf("buscar o documento cimd do cliente %s: %w", oauth.ClientValue(clientID), err)
	}
	return oauth.ClientMetadata{
		ClientID: meta.ClientID, Name: meta.Name, RedirectURIs: meta.RedirectURIs, TTL: meta.TTL,
	}, nil
}
