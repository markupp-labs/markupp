# Contas e tokens da FakeMarkuppApi, compartilhados pelas fakes de cada assunto.
class FakeMarkuppAccounts
  DEFAULT_EMAIL = "ana@markupp.dev"
  DEFAULT_PASSWORD = "senha-certa"
  DEFAULT_ACCOUNTS = [ { email: DEFAULT_EMAIL, password: DEFAULT_PASSWORD } ].freeze
  ACCESS_TOKEN_SECONDS = 900

  def initialize(accounts)
    @accounts = accounts.map.with_index(1) { |account, number| account.stringify_keys.merge("id" => "account-#{number}") }
    @access_tokens = {}
    @refresh_tokens = {}
    @issued_pairs = 0
  end

  def find_by_credentials(email, password)
    @accounts.find { |account| account["email"] == email && account["password"] == password }
  end

  # Par novo, como o do /auth/login e do /auth/refresh; os tokens seguem a ordem de emissão: access-1, refresh-1...
  def issue_pair(account)
    @issued_pairs += 1
    @access_tokens["access-#{@issued_pairs}"] = account
    @refresh_tokens["refresh-#{@issued_pairs}"] = account
    { "access_token" => "access-#{@issued_pairs}", "refresh_token" => "refresh-#{@issued_pairs}", "expires_in" => ACCESS_TOKEN_SECONDS }
  end

  def account_for_access(token, call)
    @access_tokens.fetch(token) do
      raise MarkuppApi::Unauthorized, "fake API refuses #{call} with access token #{token.inspect}, expected one of #{@access_tokens.keys}"
    end
  end

  # Rotação, como a API: o refresh usado deixa de valer e volta um par novo.
  def rotate_refresh(token)
    account = @refresh_tokens.delete(token)
    raise MarkuppApi::RefreshRejected, "fake API refuses refresh token #{token.inspect}, expected one of #{@refresh_tokens.keys}" unless account

    issue_pair(account)
  end

  def revoke_refresh(token) = @refresh_tokens.delete(token)

  def refresh_token_valid?(token) = @refresh_tokens.key?(token)

  # Cenário: o token de acesso venceu no servidor, a próxima chamada autenticada leva 401.
  def expire_access_tokens! = @access_tokens.clear

  # Cenário: a sessão foi revogada no servidor, a renovação leva invalid_refresh_token.
  def revoke_refresh_tokens! = @refresh_tokens.clear
end
