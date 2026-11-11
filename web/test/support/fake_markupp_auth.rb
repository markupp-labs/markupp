# Rotas /auth da FakeMarkuppApi: login, renovação com rotação, logout, /auth/me e aceite de convite por token.
class FakeMarkuppAuth
  PUBLIC_CALLS = [ "POST /auth/login", "POST /auth/refresh", "POST /auth/logout", "POST /auth/invites/accept", "POST /v1/accounts" ].freeze
  PASSWORD_LIMIT = 72

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

  def local_auth? = @local_auth

  # Cenário: a API demora no POST /auth/refresh até `callers` renovações chegarem juntas, ou até o timeout.
  def hold_refreshes_together!(callers, timeout:) = @refresh_gate = FakeRefreshGate.new(callers, timeout:)

  # password_login só sai quando a conta da fake o define, como o servidor que ainda não manda o campo.
  def get(path, account:, **)
    raise ArgumentError, "fake API has no route for GET #{path}, expected /auth/me" unless path == "/auth/me"

    account.slice("id", "email", "password_login")
  end

  def post(path, body, **)
    case path
    when "/auth/login" then login(body)
    when "/auth/refresh" then refresh(body)
    when "/auth/logout" then logout(body)
    when "/auth/invites/accept" then accept_invite(body)
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

      @accounts.issue_pair(account)
    end

    # Como a API: a senha é conferida antes do token, e o convite só é consumido se a conta for criada.
    def accept_invite(body)
      raise MarkuppApi::LocalAuthDisabled, "fake API has local auth disabled on POST /auth/invites/accept" unless @local_auth
      refuse_bad_password(body[:password], "POST /auth/invites/accept")

      invite = @tenants.invite_for_token(body[:token])
      account = @accounts.create(invite["email"], body[:password])
      @tenants.join(invite, account)
      @accounts.issue_pair(account)
    end

    # Como a API: a primeira conta nasce sempre; as outras só com o registro aberto, e sem email confirmado.
    def register(body)
      raise MarkuppApi::LocalAuthDisabled, "fake API has local auth disabled on POST /v1/accounts" unless @local_auth
      refuse_bad_password(body[:password], "POST /v1/accounts")
      raise MarkuppApi::RegistrationDisabled, "fake API refuses POST /v1/accounts: registration_disabled" if @registration_closed && @accounts.any?

      account = @accounts.create(body[:email].to_s.strip.downcase, body[:password], verified: false)
      @accounts.issue_pair(account)
    end

    def refuse_bad_password(password, call)
      return if password.to_s.strip.present? && password.bytesize <= PASSWORD_LIMIT

      raise MarkuppApi::InvalidRequest, "fake API refuses #{call} with a password of #{password.to_s.bytesize} bytes, expected 1 to #{PASSWORD_LIMIT}"
    end

    # Como a API: 204 sempre, exista o refresh ou não.
    def logout(body)
      @accounts.revoke_refresh(body[:refresh_token])
      nil
    end
end
