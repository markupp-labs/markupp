# Provedores de login externo da instância da FakeMarkuppApi sob /v1/instance/login-providers, como a API: só a conta com
# instance_admin lê e muda, a lista fixada por quem hospeda recusa toda escrita, o id repetido e o desconhecido são
# recusados, o issuer não muda com contas ligadas e o segredo fica guardado sem voltar. scenario: { locked:, providers: },
# providers como a API devolve, mais client_secret.
class FakeMarkuppInstanceLoginProviders
  include FakeInstanceAdminGuard

  ROUTE = "/v1/instance/login-providers".freeze
  VALID_ID = /\A[a-z0-9-]+\z/

  # Cada escrita aceita: [ verbo, id, corpo ].
  attr_reader :writes

  def initialize(scenario)
    @locked = scenario.fetch(:locked, false)
    @providers = scenario.fetch(:providers, []).map { it.stringify_keys.reverse_merge(defaults(it[:id] || it["id"])) }
    @writes = []
  end

  def get(path, account:, **)
    refuse_non_admin(account, "GET #{path}")
    { "locked" => @locked, "providers" => @providers.map { view(it) } }
  end

  def post(path, body, account:, **)
    refuse_write(account, "POST #{path}", body)
    raise MarkuppApi::LoginProviderExists, "fake API refuses POST #{path} with id #{body[:id].inspect}: login_provider_exists" if find(body[:id])

    @writes << [ :post, body[:id], body ]
    view(@providers.push(defaults(body[:id]).merge(stored(body))).last)
  end

  def put(path, body, account:, **)
    provider = find!(path)
    refuse_write(account, "PUT #{path}", body.merge(id: provider["id"]))
    refuse_issuer_change(provider, body, path)
    @writes << [ :put, provider["id"], body ]
    view(provider.merge!(stored(body)))
  end

  def delete(path, account:, **)
    provider = find!(path)
    refuse_write(account, "DELETE #{path}", {})
    @writes << [ :delete, provider["id"], nil ]
    @providers.delete(provider)
    { "removed_links" => provider["linked_accounts"] }
  end

  private
    def defaults(id)
      { "id" => id, "issuer" => "", "client_id" => "client", "client_secret" => "", "enabled" => true, "locked" => false,
        "callback_url" => "https://markupp.test/v1/auth/login-providers/#{id}/callback", "linked_accounts" => 0 }
    end

    def stored(body)
      fields = body.slice(:name, :issuer, :client_id, :enabled).stringify_keys
      body[:client_secret].present? ? fields.merge("client_secret" => body[:client_secret]) : fields
    end

    def view(provider) = provider.except("client_secret").merge("client_secret_set" => provider["client_secret"].present?)

    def find(id) = @providers.find { it["id"] == id }

    def find!(path)
      find(path.delete_prefix("#{ROUTE}/")) or raise MarkuppApi::LoginProviderNotFound, "fake API has no login provider at #{path}"
    end

    def refuse_write(account, call, body)
      refuse_non_admin(account, call)
      raise MarkuppApi::HostLocked, "fake API refuses #{call}: host_locked, expected an unlocked list" if @locked
      return if body[:name].nil? || valid?(body)

      raise MarkuppApi::InvalidLoginProvider, "fake API refuses #{call} with #{body.except(:client_secret).inspect}: invalid_login_provider"
    end

    def valid?(body)
      body[:id].to_s.match?(VALID_ID) && body[:name].present? && body[:client_id].present? &&
        (body[:issuer].to_s.start_with?("https://") || (body[:issuer].blank? && body[:id] == "github"))
    end

    def refuse_issuer_change(provider, body, path)
      return if provider["linked_accounts"].zero? || body[:issuer].to_s == provider["issuer"]

      raise MarkuppApi::LoginProviderIssuerLocked, "fake API refuses PUT #{path} with issuer #{body[:issuer].inspect}: login_provider_issuer_locked"
    end
end
