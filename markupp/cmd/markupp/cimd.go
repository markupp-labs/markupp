package main

import (
	"context"
	"errors"
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
	return cimdLimits{}
}

// cimdClientMetadataFetcher entrega ao serviço OAuth os documentos que o cimd.Fetcher
// busca e valida.
type cimdClientMetadataFetcher struct {
	fetcher *cimd.Fetcher
}

// newCIMDClientMetadataFetcher monta o fetcher com os limites e a política PublicOnly;
// extra vem depois e só existe para o teste trocar autoridades e política de IP.
func newCIMDClientMetadataFetcher(limits cimdLimits, extra ...cimd.Option) (*cimdClientMetadataFetcher, error) {
	return &cimdClientMetadataFetcher{}, nil
}

// FetchClientMetadata implementa oauth.ClientMetadataFetcher.
func (a *cimdClientMetadataFetcher) FetchClientMetadata(ctx context.Context, clientID string) (oauth.ClientMetadata, error) {
	return oauth.ClientMetadata{}, errors.New("cimd: adaptador nao implementado")
}
