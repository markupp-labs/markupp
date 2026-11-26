# Rotas /auth da FakeMarkuppApi: login, renovação com rotação, logout, /auth/me e aceite de convite por token.
class FakeMarkuppAuth
  PUBLIC_CALLS = [ "POST /v1/auth/login", "POST /v1/auth/refresh", "POST /v1/auth/logout", "POST /v1/auth/invites/accept", "POST /v1/accounts",
                   "GET /v1/accounts/policy" ].freeze
  POLICY = "/v1/accounts/policy".freeze
  PASSWORD_LIMIT = 72
  PASSWORD_MINIMUM = 15
  # Amostra da lista de senhas comuns da API.
  COMMON_PASSWORDS = %w[qazwsxedcrfvtgb 1q2w3e4r5t6y7u8i].freeze

  # Como a API para senha nova: 1 a 72 bytes, depois 15 caracteres, depois fora da lista comum.
  def self.refuse_new_password(password, call)
    unless password.to_s.strip.present? && password.bytesize <= PASSWORD_LIMIT
      raise MarkuppApi::InvalidRequest, "fake API refuses #{call} with a password of #{password.to_s.bytesize} bytes, expected 1 to #{PASSWORD_LIMIT}"
    end
    raise MarkuppApi::PasswordTooShort, "fake API refuses #{call} with #{password.length} characters, expected at least #{PASSWORD_MINIMUM}" if password.length < PASSWORD_MINIMUM
    raise MarkuppApi::CommonPassword, "fake API refuses #{call} with a common password" if COMMON_PASSWORDS.include?(password.downcase)
  end

  def initialize(accounts, tenants)
    @accounts = accounts
    @tenants = tenants
    @local_auth = true
  end

  def public?(call) = PUBLIC_CALLS.include?(call)

  # Cenário: servidor só com login externo, como MARKUPP_LOCAL_AUTH desligado.
  def disable_local_auth! = @local_auth = false

  # Cenário: servidor que só cria conta por convite, como MARKUPP_AUTH_REGISTRATION_ENABLED desligado.
  def close_registration! = @registration_closed = true

  # Cenário: o administrador da instância fechou o cadastro por email e senha (ADR-0059).
  def close_password_registration! = @password_registration_closed = true

  def local_auth? = @local_auth

  # Como a API: o interruptor geral de quem hospeda.
  def registration_enabled? = !@registration_closed

  def password_registration_open? = registration_enabled? && @local_auth && !@password_registration_closed

  # Cenário: a API demora no POST /v1/auth/refresh até `callers` renovações chegarem juntas, ou até o timeout.
  def hold_refreshes_together!(callers, timeout:) = @refresh_gate = FakeRefreshGate.new(callers, timeout:)

  # password_login e instance_admin só saem quando a conta da fake os define, como o servidor que ainda não manda o campo.
  def get(path, account:, **)
    return { "registration_enabled" => registration_enabled?, "password_registration_open" => password_registration_open? } if path == POLICY
    raise ArgumentError, "fake API has no route for GET #{path}, expected /v1/auth/me or #{POLICY}" unless path == "/v1/auth/me"

    account.slice("id", "email", "password_login", "instance_admin")
  end

  def post(path, body, **)
    case path
    when "/v1/auth/login" then login(body)
    when "/v1/auth/refresh" then refresh(body)
    when "/v1/auth/logout" then logout(body)
    when "/v1/auth/invites/accept" then accept_invite(body)
    when "/v1/accounts" then register(body)
    else raise ArgumentError, "fake API has no route for POST #{path}, expected one of #{PUBLIC_CALLS}"
    end
  end

  private
    def refresh(body)
      @refresh_gate&.pass
      @accounts.rotate_refresh(body[:refresh_token])
    end

    def login(body)
      raise MarkuppApi::LocalAuthDisabled, "fake API has local auth disabled on POST /auth/login" unless @local_auth
      raise MarkuppApi::InvalidRequest, "fake API refuses POST /auth/login with #{body.inspect}, expected email and password" if body.values_at(:email, :password).any?(&:blank?)

      account = @accounts.find_by_credentials(body[:email], body[:password])
      raise MarkuppApi::InvalidCredentials, "fake API refuses POST /auth/login for #{body[:email].inspect}, expected a known email and its password" unless account
      raise MarkuppApi::AccountBlocked, "fake API refuses POST /auth/login for #{body[:email].inspect}: account_blocked" if account["blocked"]

      @accounts.issue_pair(account)
    end

    # Como a API: a senha é conferida antes do token, e o convite só é consumido se a conta for criada.
    def accept_invite(body)
      raise MarkuppApi::LocalAuthDisabled, "fake API has local auth disabled on POST /auth/invites/accept" unless @local_auth
      self.class.refuse_new_password(body[:password], "POST /v1/auth/invites/accept")

      invite = @tenants.invite_for_token(body[:token])
      account = @accounts.create(invite["email"], body[:password])
      @tenants.join(invite, account)
      @accounts.issue_pair(account)
    end

    # Como a API: a primeira conta nasce sempre; as outras só com o registro aberto, e sem email confirmado.
    def register(body)
      raise MarkuppApi::LocalAuthDisabled, "fake API has local auth disabled on POST /v1/accounts" unless @local_auth
      self.class.refuse_new_password(body[:password], "POST /v1/accounts")
      raise MarkuppApi::RegistrationDisabled, "fake API refuses POST /v1/accounts: registration_disabled" if !password_registration_open? && @accounts.any?

      account = @accounts.create(body[:email].to_s.strip.downcase, body[:password], verified: false)
      @accounts.issue_pair(account)
    end

    # Como a API: 204 sempre, exista o refresh ou não.
    def logout(body)
      @accounts.revoke_refresh(body[:refresh_token])
      nil
    end
end
