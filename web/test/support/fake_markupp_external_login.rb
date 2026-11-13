# Rotas de login externo da FakeMarkuppApi (ADR-0054): provedores, tentativa, login, cadastro e identidades da conta.
# Como a API, só troca o código com o verifier e o nonce da tentativa que emitiu. O código diz quem o provedor devolve:
# CODES leva cada código à identidade, e código fora dele é recusado como login_rejected.
class FakeMarkuppExternalLogin
  PROVIDERS = "/v1/login-providers".freeze
  PUBLIC_PATHS = [ "/v1/auth/external-login", "/v1/accounts/external" ].freeze
  IDENTITIES = "/v1/me/identities".freeze
  DEFAULT_PROVIDERS = [ { "id" => "github", "name" => "GitHub" } ].freeze
  # Identidade nova, ainda sem conta, e a que já é da conta padrão quando o teste a vincula.
  CODES = {
    "codigo-novo" => { subject: "42", email: "novo@markupp.dev" },
    "codigo-ana" => { subject: "7", email: FakeMarkuppAccounts::DEFAULT_EMAIL }
  }.freeze

  def self.handles?(path) = path.start_with?(PROVIDERS, IDENTITIES) || PUBLIC_PATHS.include?(path)

  def initialize(providers, accounts, auth)
    @providers = providers
    @accounts = accounts
    @auth = auth
    @attempts = {}
    @identities = []
  end

  def public?(call) = call.split(" ", 2).last.then { it.start_with?(PROVIDERS) || PUBLIC_PATHS.include?(it) }

  # Cenário: o provedor não responde, como o IdP fora do ar.
  def take_provider_down! = @provider_down = true

  # Cenário: a identidade do código já é da conta de email.
  def link_identity!(code, email)
    @identities << { "id" => "identity-#{@identities.size + 1}", "provider" => "github", "subject" => CODES.fetch(code)[:subject],
                     "account" => @accounts.find_by_email(email), "created_at" => "2026-11-12T10:00:00Z" }
  end

  def get(path, account:, **)
    return @providers if path == PROVIDERS
    raise ArgumentError, "fake API has no route for GET #{path}, expected #{PROVIDERS} or #{IDENTITIES}" unless path == IDENTITIES

    identities_of(account).map { it.slice("id", "provider", "created_at") }
  end

  def post(path, body, account:, **)
    case path
    when "/v1/auth/external-login" then @accounts.issue_pair(linked_account(identity_for(body)))
    when "/v1/accounts/external" then register(identity_for(body))
    when IDENTITIES then link(identity_for(body), account)
    else attempt(path)
    end
  end

  def delete(path, account:, **)
    identity = identities_of(account).find { path == "#{IDENTITIES}/#{it["id"]}" }
    raise MarkuppApi::NotFound, "fake API has no identity at DELETE #{path}" unless identity
    raise MarkuppApi::LastIdentity, "fake API refuses DELETE #{path}: last_identity" if account["password"].nil? && identities_of(account).one?

    @identities.delete(identity)
    nil
  end

  private
    def attempt(path)
      provider = @providers.find { path == "#{PROVIDERS}/#{it["id"]}/attempts" }
      raise MarkuppApi::NotFound, "fake API has no login provider at POST #{path}" unless provider
      raise MarkuppApi::ProviderUnavailable, "fake API provider #{provider["id"]} is down" if @provider_down

      number = @attempts.size + 1
      @attempts["verifier-#{number}"] = "nonce-#{number}"
      { "authorization_url" => "https://#{provider["id"]}.example/authorize?state=state-#{number}",
        "state" => "state-#{number}", "code_verifier" => "verifier-#{number}", "nonce" => "nonce-#{number}" }
    end

    # Como a API: código, verifier e nonce precisam ser os da tentativa.
    def identity_for(body)
      identity = CODES[body[:code]]
      unless identity && @attempts[body[:code_verifier]] == body[:nonce] && @attempts[body[:code_verifier]]
        raise MarkuppApi::LoginRejected, "fake API rejects #{body.inspect}: login_rejected"
      end

      identity.merge(provider: body[:provider])
    end

    def linked_account(identity)
      linked = find_identity(identity)
      raise MarkuppApi::IdentityNotLinked, "fake API has no account for #{identity}: identity_not_linked" unless linked

      linked["account"]
    end

    def register(identity)
      raise MarkuppApi::IdentityAlreadyLinked, "fake API refuses #{identity}: identity_already_linked" if find_identity(identity)
      raise MarkuppApi::RegistrationDisabled, "fake API refuses the external account: registration_disabled" if @auth.registration_closed?

      account = @accounts.create(identity[:email], nil)
      account["password_login"] = false
      store(identity, account)
      @accounts.issue_pair(account)
    end

    def link(identity, account)
      raise MarkuppApi::IdentityAlreadyLinked, "fake API refuses #{identity}: identity_already_linked" if find_identity(identity)

      store(identity, account).slice("id", "provider", "created_at")
    end

    def store(identity, account)
      @identities << { "id" => "identity-#{@identities.size + 1}", "provider" => identity[:provider], "subject" => identity[:subject],
                       "account" => account, "created_at" => "2026-11-12T10:00:00Z" }
      @identities.last
    end

    def find_identity(identity) = @identities.find { it["provider"] == identity[:provider] && it["subject"] == identity[:subject] }

    def identities_of(account) = @identities.select { it["account"]["id"] == account["id"] }
end
