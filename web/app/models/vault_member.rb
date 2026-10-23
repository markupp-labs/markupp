# Membro de um cofre, como GET /v1/vaults/{id}/members devolve; role é o papel dele no tenant. Remover a si mesmo é
# sair do cofre, a única escrita que o viewer faz.
VaultMember = Data.define(:id, :email, :role, :joined_at) do
  extend V1Listing

  # Colega do tenant que ainda pode entrar no cofre, como GET /v1/users devolve.
  self::Candidate = Data.define(:id, :email)

  def self.all(api, vault_id) = list_from(api, "vaults", vault_id, "members")

  def self.add(api, vault_id, user_id) = api.post(MarkuppApi.path("v1", "vaults", vault_id, "members"), user_id:)

  def self.remove(api, vault_id, user_id) = api.delete(MarkuppApi.path("v1", "vaults", vault_id, "members", user_id))

  # Os colegas do tenant fora de members, na ordem da API (por email).
  def self.candidates(api, members)
    ids = members.map(&:id)
    api.get(MarkuppApi.path("v1", "users")).filter_map { self::Candidate.new(**it.symbolize_keys.slice(:id, :email)) unless ids.include?(it["id"]) }
  end

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, joined_at: Time.zone.parse(fields.fetch(:joined_at)))
  end
end
