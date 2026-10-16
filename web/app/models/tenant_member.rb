# Membro do tenant ativo, como GET /users devolve. Posto e último owner quem decide é a API; o painel só espelha.
TenantMember = Data.define(:id, :email, :role, :joined_at) do
  def self.all(api) = api.get("/users").map { from_api(it) }

  # Métodos de classe porque o controller só tem o id.
  def self.change_role(api, id, role) = api.patch(member_path(id), role:)

  def self.remove(api, id) = api.delete(member_path(id))

  def self.member_path(id) = MarkuppApi.path("users", id)

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, joined_at: Time.zone.parse(fields.fetch(:joined_at)))
  end

  # Quem age só mexe em quem está no próprio posto ou abaixo.
  def changeable_by?(tenant) = tenant.assignable_roles.include?(role)
end
