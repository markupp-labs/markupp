# Vínculo da conta com um tenant, como GET /me/tenants devolve: o papel vem daqui a cada página, sem cache.
# notes_count e last_note_at são estatísticas que o servidor pode não mandar: sem elas, ficam nil.
Tenant = Data.define(:id, :name, :role, :notes_count, :last_note_at) do
  # Mesma ordem do servidor, do posto mais alto ao mais baixo.
  self::ROLES = %w[owner admin member viewer].freeze

  def self.mine(api) = api.get("/me/tenants").map { from_api(it) }

  # Quem cria é owner; nome recusado pela API levanta InvalidRequest, a regra fica no servidor.
  def self.create(api, name) = from_api(api.post("/tenants", name:))

  # Apaga de vez o tenant e as notas dele; só o owner pode, e a API decide.
  def self.delete(api, id) = api.delete(MarkuppApi.path("tenants", id))

  def self.from_api(attributes)
    fields = attributes.symbolize_keys.slice(*members)
    new(**fields, last_note_at: fields[:last_note_at] && Time.zone.parse(fields[:last_note_at]))
  end

  def initialize(notes_count: nil, last_note_at: nil, **) = super

  # viewer só lê notas; os outros papéis escrevem.
  def writer? = role != "viewer"

  # Só o owner apaga o tenant.
  def owner? = role == "owner"

  # owner e admin administram membros e convites.
  def admin? = %w[owner admin].include?(role)

  # Papéis que este vínculo concede ou altera: o próprio e os de posto menor.
  def assignable_roles
    rank = Tenant::ROLES.index(role) or raise ArgumentError, "unknown role #{role.inspect}, expected one of #{Tenant::ROLES.join(', ')}"
    Tenant::ROLES.drop(rank)
  end

  # Tenant ativo: o da sessão enquanto estiver na lista; sem ele, o único que houver.
  def self.pick(tenants, id) = tenants.find { it.id == id } || (tenants.sole if tenants.one?)
end
