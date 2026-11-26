# Autorização ativa da conta, como GET /v1/me/authorizations devolve: um cliente (markupp-mcp, markupp-cli, um agente
# remoto) com os escopos que a conta concedeu. Revogar derruba os tokens desse cliente e deixa os outros.
# ip, user_agent e last_used_at são do último uso do cliente (ADR-0061); last_used_at é nil sem uso registrado.
OAuthGrant = Data.define(:id, :client_name, :scopes, :created_at, :ip, :user_agent, :last_used_at) do
  extend V1Listing

  def self.all(api) = list_from(api, "me", "authorizations")

  def self.revoke(api, id) = api.delete(MarkuppApi.path("v1", "me", "authorizations", id))

  def self.from_api(attributes)
    fields = attributes.symbolize_keys
    used = fields[:last_used_at]
    new(id: fields.fetch(:id), client_name: fields.fetch(:client_name), scopes: fields.fetch(:scopes),
        created_at: Time.zone.parse(fields.fetch(:created_at)), ip: fields[:ip].to_s, user_agent: fields[:user_agent].to_s,
        last_used_at: used && Time.zone.parse(used))
  end
end
