# Rotas /auth da FakeMarkuppApi: login, renovação com rotação, logout e /auth/me.
class FakeMarkuppAuth
  PUBLIC_CALLS = [ "POST /auth/login", "POST /auth/refresh", "POST /auth/logout" ].freeze

  def initialize(accounts)
    @accounts = accounts
    @local_auth = true
  end

  def public?(call) = PUBLIC_CALLS.include?(call)

  # Cenário: servidor só com login externo, como MARKUPP_LOCAL_AUTH desligado.
  def disable_local_auth! = @local_auth = false

  def get(path, account:)
    raise ArgumentError, "fake API has no route for GET #{path}, expected /auth/me" unless path == "/auth/me"

    account.slice("id", "email")
  end

  def post(path, body, **)
    case path
    when "/auth/login" then login(body)
    when "/auth/refresh" then @accounts.rotate_refresh(body[:refresh_token])
    when "/auth/logout" then logout(body)
    else raise ArgumentError, "fake API has no route for POST #{path}, expected one of #{PUBLIC_CALLS}"
    end
  end

  private
    def login(body)
      raise MarkuppApi::LocalAuthDisabled, "fake API has local auth disabled on POST /auth/login" unless @local_auth
      raise MarkuppApi::InvalidRequest, "fake API refuses POST /auth/login with #{body.inspect}, expected email and password" if body.values_at(:email, :password).any?(&:blank?)

      account = @accounts.find_by_credentials(body[:email], body[:password])
      raise MarkuppApi::InvalidCredentials, "fake API refuses POST /auth/login for #{body[:email].inspect}, expected a known email and its password" unless account

      @accounts.issue_pair(account)
    end

    # Como a API: 204 sempre, exista o refresh ou não.
    def logout(body)
      @accounts.revoke_refresh(body[:refresh_token])
      nil
    end
end
