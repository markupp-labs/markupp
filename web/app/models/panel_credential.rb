# Credencial do painel na sessão do Rails, um cookie criptografado e HttpOnly: os tokens nunca vão ao HTML nem ao JS.
class PanelCredential
  def initialize(session, api_factory)
    @session = session
    @api_factory = api_factory
  end

  # pair é a resposta de /auth/login ou /auth/refresh: access_token, refresh_token e expires_in em segundos.
  def sign_in(pair)
    @session[:access_token] = pair.fetch("access_token")
    @session[:refresh_token] = pair.fetch("refresh_token")
    @session[:access_expires_at] = Time.current.to_i + pair.fetch("expires_in")
  end

  # account é a resposta de /auth/me, guardada para o layout não chamar a API a cada página.
  def account=(account)
    @session[:account_id] = account.fetch("id")
    @session[:account_email] = account.fetch("email")
    @session[:password_login] = account["password_login"]
  end

  def sign_out = @session.destroy

  def signed_in? = refresh_token.present?

  def refresh_token = @session[:refresh_token]

  def tenant_id = @session[:tenant_id]

  def tenant_id=(id)
    @session[:tenant_id] = id
  end

  def account_id = @session[:account_id]

  def account_email = @session[:account_email]

  # Só false esconde a troca de senha: servidor que ainda não manda o campo mantém o botão.
  def password_login? = @session[:password_login] != false

  def access_expired? = Time.current.to_i >= @session[:access_expires_at].to_i

  # Cliente das páginas logadas: renova o token sozinho.
  def api = RefreshingMarkuppApi.new(self)

  def bearer_api = @api_factory.call(access_token: @session[:access_token], tenant_id:)

  # Sem token nem tenant: login, renovação, logout e rotas de token por email.
  def public_api = @api_factory.call
end
