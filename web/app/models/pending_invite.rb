# Convite que a conta recebeu, como GET /me/invites devolve. A API só lista convites de conta com email verificado.
PendingInvite = Data.define(:id, :tenant_id, :tenant_name, :role, :expires_at) do
  def self.mine(api) = api.get("/me/invites").map { from_api(it) }

  # Devolve o tenant em que a conta entrou, com o papel do convite.
  def self.accept(api, id) = Tenant.from_api(api.post(answer_path(id, "accept"), {}))

  def self.decline(api, id) = api.post(answer_path(id, "decline"), {})

  def self.answer_path(id, answer) = MarkuppApi.path("me", "invites", id, answer)

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, expires_at: Time.zone.parse(fields.fetch(:expires_at)))
  end
end
