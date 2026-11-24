# Embedding da instância da FakeMarkuppApi sob /v1/instance/embedding, como a API: só a conta com instance_admin lê e
# grava, a chave fica guardada e nunca volta, a escolha travada por quem hospeda recusa o PUT e o remoto pede URL http(s)
# e modelo, e trocar para outro endpoint remoto sem mandar a chave de novo é recusado; o local e o none não usam chave e apagam a guardada. scenario: { provider:, url:, model:, api_key:, locked:, host_provider:, model_status: }.
class FakeMarkuppInstanceEmbedding
  include FakeInstanceAdminGuard

  ROUTE = "/v1/instance/embedding".freeze
  KEYLESS_PROVIDERS = %w[local none].freeze

  attr_reader :updates

  def initialize(scenario)
    @choice = { provider: "none", url: "", model: "", api_key: "" }.merge(scenario.slice(:provider, :url, :model, :api_key))
    @scenario = scenario
    @updates = []
  end

  def get(path, account:, **)
    refuse_non_admin(account, "GET #{path}")
    view
  end

  def put(path, body, account:, **)
    call = "PUT #{path}"
    refuse_non_admin(account, call)
    raise MarkuppApi::HostLocked, "fake API refuses #{call}: host_locked, expected an unlocked choice" if @scenario[:locked]

    refuse_invalid(body, call)
    refuse_key_reuse(body, call)
    @updates << body
    @choice = @choice.merge(body.slice(:provider, :url, :model, :api_key).compact)
    @choice[:api_key] = "" if KEYLESS_PROVIDERS.include?(body[:provider]) && !body.key?(:api_key)
    view
  end

  private
    def refuse_invalid(body, call)
      return unless body[:provider] == "openai-compat"
      return if body[:url].to_s.match?(%r{\Ahttps?://.+}) && body[:model].present?

      raise MarkuppApi::InvalidRequest, "fake API refuses #{call} with url #{body[:url].inspect}: invalid_request, expected http(s) URL and model"
    end

    def refuse_key_reuse(body, call)
      return if body.key?(:api_key) || @choice[:api_key].blank? || KEYLESS_PROVIDERS.include?(body[:provider])
      return if body[:provider] == @choice[:provider] && body[:url].to_s == @choice[:url]

      raise MarkuppApi::ApiKeyRequired, "fake API refuses #{call} with url #{body[:url].inspect}: api_key_required, expected api_key sent again"
    end

    def view
      { "provider" => @choice[:provider], "url" => @choice[:url], "model" => @choice[:model], "api_key_set" => @choice[:api_key].present?,
        "locked" => @scenario.fetch(:locked, false), "host_provider" => @scenario.fetch(:host_provider, ""),
        "model_status" => @scenario[:model_status]&.stringify_keys }
    end
end
