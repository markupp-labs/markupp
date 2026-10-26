# Notificação da conta, como GET /v1/me/notifications devolve (ADR-0049). Hoje a única fonte é o convite pendente, que
# vem com os campos dele e se responde pelas rotas de my_invites.
AccountNotification = Data.define(:kind, :invite) do
  def self.feed(api) = AccountNotification::Feed.from_api(api.get(MarkuppApi.path("v1", "me", "notifications")))

  def self.from_api(attributes) = new(kind: attributes.fetch("kind"), invite: PendingInvite.from_api(attributes))
end

# A lista da conta e quantas pedem resposta. O tipo que o painel ainda não sabe mostrar (API mais nova) fica fora da
# lista; o unread é o da API.
AccountNotification::Feed = Data.define(:items, :unread) do
  def self.from_api(body)
    known = body.fetch("items").select { it["kind"] == "invite" }
    new(items: known.map { AccountNotification.from_api(it) }, unread: body.fetch("unread"))
  end
end
