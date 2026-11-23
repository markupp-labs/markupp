# Tentativa de login externo guardada na sessão do painel (cookie criptografado) entre a ida ao provedor e a volta. O
# servidor gera state, verifier e nonce; o painel confere o state na volta e devolve o resto ao servidor com o código.
class ExternalLoginAttempt
  PURPOSES = %w[login signup link].freeze
  SESSION_KEY = "external_login".freeze

  # Pede a tentativa ao servidor, guarda na sessão e devolve a URL do provedor, que só pode ser http(s) absoluta.
  def self.start(session, public_api, provider:, purpose:)
    attempt = public_api.post(MarkuppApi.path("v1", "login-providers", provider, "attempts"), {})
    session[SESSION_KEY] = { "provider" => provider, "purpose" => purpose, **attempt.slice("state", "code_verifier", "nonce") }
    authorization_url(attempt.fetch("authorization_url"))
  end

  def self.authorization_url(url)
    return url if URI.parse(url).then { it.is_a?(URI::HTTP) && it.host.present? }

    raise MarkuppApi::Error, "login provider authorization_url #{url.inspect} is not an absolute http(s) URL"
  rescue URI::InvalidURIError
    raise MarkuppApi::Error, "login provider authorization_url #{url.inspect} is not a URL"
  end

  # Tira da sessão a tentativa, que vale uma vez só; nil quando não há ou quando provedor ou state não batem.
  def self.take(session, provider:, state:)
    stored = session.delete(SESSION_KEY)
    return unless stored && stored["provider"] == provider
    return unless ActiveSupport::SecurityUtils.secure_compare(stored["state"].to_s, state.to_s)

    new(stored)
  end

  def initialize(stored)
    @stored = stored
  end

  def purpose = @stored["purpose"]

  def provider = @stored["provider"]

  # O corpo de /v1/auth/external-login, /v1/accounts/external e /v1/me/identities.
  def callback_body(code) = { provider:, code:, code_verifier: @stored["code_verifier"], nonce: @stored["nonce"] }
end
