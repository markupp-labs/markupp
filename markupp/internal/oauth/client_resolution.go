package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// metadataDocumentShape descreve o documento esperado quando a busca do CIMD falha.
const metadataDocumentShape = "um Client ID Metadata Document acessivel e valido (draft-ietf-oauth-client-id-metadata-document)"

// resolveClient acha o cliente do client_id: por CIMD quando ele é URL https e há
// fetcher, senão pelo cadastro do repositório.
func (s *Service) resolveClient(ctx context.Context, clientID string) (Client, error) {
	if s.metadata == nil || !isHTTPSURL(clientID) {
		return s.registeredClient(ctx, clientID)
	}
	return s.cimdClient(ctx, clientID)
}

// isHTTPSURL diz se o client_id tem a forma de URL https com host.
func isHTTPSURL(clientID string) bool {
	parsed, err := url.Parse(clientID)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

// registeredClient busca o cliente cadastrado, por CIMD ou DCR, sem buscar documento.
func (s *Service) registeredClient(ctx context.Context, clientID string) (Client, error) {
	client, err := s.repo.GetClient(ctx, clientID)
	if errors.Is(err, ErrClientNotFound) {
		return Client{}, &ProtocolError{Code: errInvalidRequest, Description: "client_id " + ClientValue(clientID) + " nao registrado; esperado " + registeredClientShape}
	}
	if err != nil {
		return Client{}, fmt.Errorf("oauth: buscando o cliente %s: %w", ClientValue(clientID), err)
	}
	return client, nil
}

// cimdClient usa o documento em cache ainda válido ou busca e grava um novo. Documento
// vencido com a busca falhando recusa o pedido, sem cair na cópia vencida.
func (s *Service) cimdClient(ctx context.Context, clientID string) (Client, error) {
	now := s.clock()
	client, err := s.repo.GetFreshCIMDClient(ctx, clientID, now)
	if err == nil {
		return client, nil
	}
	if !errors.Is(err, ErrClientNotFound) {
		return Client{}, fmt.Errorf("oauth: lendo o cache do cliente cimd %s: %w", ClientValue(clientID), err)
	}
	return s.fetchCIMDClient(ctx, clientID, now)
}

// fetchCIMDClient busca o documento e grava o cliente até o fim do TTL dele. O erro da
// busca vai só para o log, como causa interna, para não servir de oráculo da rede interna.
func (s *Service) fetchCIMDClient(ctx context.Context, clientID string, now time.Time) (Client, error) {
	metadata, err := s.metadata.FetchClientMetadata(ctx, clientID)
	if err != nil {
		return Client{}, unusableMetadataError(clientID, err)
	}
	client := Client{ID: clientID, Name: metadata.Name, RedirectURIs: metadata.RedirectURIs}
	if err := s.repo.SaveCIMDClient(ctx, client, now.Add(metadata.TTL)); err != nil {
		return Client{}, fmt.Errorf("oauth: gravando o cliente cimd %s: %w", ClientValue(clientID), err)
	}
	return client, nil
}

// unusableMetadataError é a recusa genérica do documento CIMD, igual para toda falha;
// cause fica só para o log.
func unusableMetadataError(clientID string, cause error) error {
	recusa := &ProtocolError{Code: errInvalidRequest, Description: "documento de metadados do client_id " + ClientValue(clientID) + " inacessivel ou invalido; esperado " + metadataDocumentShape}
	return recusa.WithInternalCause(cause)
}
