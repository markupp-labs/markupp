# Vínculo da conta com um tenant, como GET /me/tenants devolve: o papel vem daqui a cada página, sem cache.
Tenant = Data.define(:id, :name, :role) do
  # Mesma ordem do servidor, do posto mais alto ao mais baixo.
  self::ROLES = %w[owner admin member viewer].freeze

  def self.mine(api) = api.get("/me/tenants").map { from_api(it) }

  # Quem cria é owner; nome recusado pela API levanta InvalidRequest, a regra fica no servidor.
  def self.create(api, name) = from_api(api.post("/tenants", name:))

  def self.from_api(attributes) = new(**attributes.symbolize_keys.slice(*members))

  # Tenant ativo: o da sessão enquanto estiver na lista; sem ele, o único que houver.
  # viewer só lê notas; os outros papéis escrevem.
  def writer? = role != "viewer"

  # owner e admin administram membros e convites.
  def admin? = %w[owner admin].include?(role)

  # Papéis que este vínculo concede ou altera: o próprio e os de posto menor.
  def assignable_roles
    rank = Tenant::ROLES.index(role) or raise ArgumentError, "unknown role #{role.inspect}, expected one of #{Tenant::ROLES.join(', ')}"
    Tenant::ROLES.drop(rank)
  end

  def self.pick(tenants, id) = tenants.find { it.id == id } || (tenants.sole if tenants.one?)
end
