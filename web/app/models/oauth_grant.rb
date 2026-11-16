# Autorização ativa da conta, como GET /v1/me/authorizations devolve: um cliente (markupp-mcp, markupp-cli, um agente
# remoto) com os escopos que a conta concedeu. Revogar derruba os tokens desse cliente e deixa os outros.
OAuthGrant = Data.define(:id, :client_name, :scopes, :created_at) do
  extend V1Listing
  extend TimestampedFromApi

  def self.all(api) = list_from(api, "me", "authorizations")

  def self.revoke(api, id) = api.delete(MarkuppApi.path("v1", "me", "authorizations", id))

  def self.from_api(attributes) = timestamped_from(attributes, :created_at)
end
