# Cofre do tenant ativo de que a conta é membro, como GET /v1/vaults devolve. Quem pode criar, renomear e apagar quem
# decide é a API (viewer não escreve); o painel só espelha.
Vault = Data.define(:id, :name, :created_at) do
  extend V1Listing

  def self.mine(api) = list_from(api, "vaults")

  # Quem cria vira membro. O nome vai sem os espaços das pontas; nome recusado levanta InvalidName.
  def self.create(api, name) = from_api(api.post(MarkuppApi.path("v1", "vaults"), name: name.to_s.strip))

  def self.rename(api, id, name) = from_api(api.patch(vault_path(id), name: name.to_s.strip))

  # Apaga de vez o cofre e as notas dele.
  def self.delete(api, id) = api.delete(vault_path(id))

  def self.vault_path(id) = MarkuppApi.path("v1", "vaults", id)

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, created_at: Time.zone.parse(fields.fetch(:created_at)))
  end

  # Cofre ativo: o da sessão enquanto estiver na lista; sem ele, o primeiro que houver.
  def self.pick(vaults, id) = vaults.find { it.id == id } || vaults.first
end
