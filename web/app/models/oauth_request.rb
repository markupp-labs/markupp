# Pedido de autorização OAuth que o servidor manda o painel decidir, como GET /v1/oauth/requests/{id} devolve. Pedido
# inexistente, vencido ou já decidido levanta MarkuppApi::OAuthRequestNotFound.
OAuthRequest = Data.define(:id, :client_name, :redirect_host, :scopes, :already_authorized) do
  def self.find(api, id) = from_api(api.get(path(id)))

  # Aprova ou recusa e devolve o endereço do cliente para onde o navegador volta, com o código ou com a recusa.
  def self.decide(api, id, decision)
    raise ArgumentError, "Unknown OAuth decision #{decision.inspect}, expected approve or deny" unless %w[approve deny].include?(decision)

    api.post(path(id, decision), {}).fetch("redirect_to")
  end

  def self.path(id, *decision) = MarkuppApi.path("v1", "oauth", "requests", id, *decision)

  def self.from_api(attributes) = new(**attributes.symbolize_keys.slice(*members))
end
