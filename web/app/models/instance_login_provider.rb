# Provedor de login externo da instância, como GET /v1/instance/login-providers devolve: GitHub, Google ou outro OIDC.
# O segredo nunca volta, só se há um guardado; callback_url é o endereço que o administrador registra no provedor.
InstanceLoginProvider = Data.define(:id, :name, :issuer, :client_id, :client_secret_set, :enabled, :locked, :callback_url,
                                    :linked_accounts) do
  # Começos prontos do formulário de criação: id, nome e issuer de quem é comum.
  self::PRESETS = {
    "github" => { id: "github", name: "GitHub", issuer: "" },
    "google" => { id: "google", name: "Google", issuer: "https://accounts.google.com" }
  }.freeze

  # A lista e se quem hospeda a fixou.
  self::Listing = Data.define(:locked, :providers)

  def self.fetch(api)
    listing = api.get(path)
    self::Listing.new(locked: listing.fetch("locked"), providers: listing.fetch("providers").map { from_api(it) })
  end

  def self.create(api, **fields) = from_api(api.post(path, fields))

  # client_secret nil mantém o guardado; o id não muda.
  def self.update(api, id, **fields) = from_api(api.put(path(id), fields.except(:id)))

  def self.destroy(api, id) = api.delete(path(id)).fetch("removed_links")

  def self.path(*id) = MarkuppApi.path("v1", "instance", "login-providers", *id)

  # Formulário de criação ainda sem provedor: em branco, ou com o preset e o que foi digitado.
  def self.draft(preset: nil, **fields)
    new(id: "", name: "", issuer: "", client_id: "", client_secret_set: false, enabled: true, locked: false, callback_url: nil,
        linked_accounts: 0, **self::PRESETS.fetch(preset, {}), **fields.slice(:id, :name, :issuer, :client_id, :enabled))
  end

  def self.from_api(attributes)
    new(**members.to_h { [ it, attributes.fetch(it.to_s) ] })
  end
end
