# Método de cadastro da instância, como GET /v1/instance/registration-methods devolve (ADR-0059): email e senha (local)
# ou um provedor de login externo, se ele cria conta nova e se quem hospeda travou o valor.
RegistrationMethod = Data.define(:id, :name, :open, :locked) do
  def self.all(api) = api.get(path).map { from_api(it) }

  def self.update(api, id, open:) = from_api(api.patch(path(id), { open: }))

  def self.path(*id) = MarkuppApi.path("v1", "instance", "registration-methods", *id)

  def self.from_api(attributes)
    new(id: attributes.fetch("id"), name: attributes.fetch("name"), open: attributes.fetch("open"), locked: attributes.fetch("locked"))
  end

  def password? = id == "local"
end
