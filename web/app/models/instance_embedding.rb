# Escolha do provedor de embedding da instância, como GET /v1/instance/embedding devolve (ADR-0057): o provedor, a URL e
# o modelo do endpoint remoto, se há chave (nunca a chave), se quem hospeda travou a escolha e o estado do modelo.
InstanceEmbedding = Data.define(:provider, :url, :model, :api_key_set, :locked, :host_provider, :model_status) do
  self::PROVIDERS = %w[none local openai-compat].freeze
  self::PATH = "/v1/instance/embedding".freeze

  def self.fetch(api) = from_api(api.get(self::PATH))

  # api_key nil mantém a chave guardada; vazia a apaga.
  def self.save(api, provider:, url:, model:, api_key: nil)
    from_api(api.put(self::PATH, { provider:, url:, model:, api_key: }.compact))
  end

  def self.from_api(attributes)
    status = attributes["model_status"]
    new(provider: attributes["provider"], url: attributes["url"], model: attributes["model"], api_key_set: attributes["api_key_set"],
        locked: attributes["locked"], host_provider: attributes["host_provider"], model_status: status && self::ModelStatus.from_api(status))
  end

  # Estado do modelo que o indexador de embedding informou.
  self::ModelStatus = Data.define(:state, :error, :changed_at) do
    def self.from_api(attributes)
      new(state: attributes["state"], error: attributes["error"], changed_at: attributes["changed_at"] && Time.zone.parse(attributes["changed_at"]))
    end
  end
end
