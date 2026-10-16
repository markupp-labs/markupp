# Vínculo da conta com um tenant, como GET /me/tenants devolve: o papel vem daqui a cada página, sem cache.
Tenant = Data.define(:id, :name, :role) do
  def self.mine(api) = api.get("/me/tenants").map { from_api(it) }

  # Quem cria é owner; nome recusado pela API levanta InvalidRequest, a regra fica no servidor.
  def self.create(api, name) = from_api(api.post("/tenants", name:))

  def self.from_api(attributes) = new(**attributes.symbolize_keys.slice(*members))

  # Tenant ativo: o da sessão enquanto estiver na lista; sem ele, o único que houver.
  def self.pick(tenants, id) = tenants.find { it.id == id } || (tenants.sole if tenants.one?)
end
