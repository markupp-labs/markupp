// Package cimd busca e valida o Client ID Metadata Document
// (draft-ietf-oauth-client-id-metadata-document): o client_id do agente é uma URL
// https que publica os próprios metadados. A busca recusa destinos fora da internet
// pública para não virar vetor de SSRF.
package cimd
