# Convite emitido pelo tenant ativo, como GET /users/invites devolve: a lista inclui os aceitos e os vencidos.
TenantInvite = Data.define(:id, :email, :role, :expires_at, :accepted_at) do
  # Pendentes primeiro, cada grupo na ordem da API.
  def self.all(api) = api.get("/users/invites").map { from_api(it) }.partition(&:pending?).flatten

  # Devolve o convite e o token, que só vem quando o servidor não manda email.
  def self.issue(api, email:, role:)
    answer = api.post("/users/invites", email:, role:)
    [ from_api(answer), answer["token"] ]
  end

  # Método de classe porque o controller só tem o id; convite já respondido levanta InviteNotFound.
  def self.revoke(api, id) = api.delete(MarkuppApi.path("users", "invites", id))

  def self.from_api(attributes)
    fields = attributes.symbolize_keys
    new(**fields.slice(:id, :email, :role), expires_at: Time.zone.parse(fields.fetch(:expires_at)),
        accepted_at: fields[:accepted_at] && Time.zone.parse(fields[:accepted_at]))
  end

  def status
    return "accepted" if accepted_at

    expires_at.past? ? "expired" : "pending"
  end

  def pending? = status == "pending"
end
