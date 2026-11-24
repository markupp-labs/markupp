# Provedor externo de login que o servidor oferece (ADR-0054), como GET /v1/login-providers devolve.
# registration_open diz se o provedor cria conta nova (ADR-0059); o login por ele não depende disso.
LoginProvider = Data.define(:id, :name, :registration_open) do
  # Lista vazia quando o servidor não responde: o login por email e senha segue sem os botões.
  def self.all(public_api)
    public_api.get(MarkuppApi.path("v1", "login-providers")).map do
      new(id: it.fetch("id"), name: it.fetch("name"), registration_open: it.fetch("registration_open", true))
    end
  rescue MarkuppApi::Error => error
    Rails.logger.warn({ event: "markupp_login_providers_unreachable", message: error.message }.to_json)
    []
  end

  def initialize(id:, name:, registration_open: true) = super

  # Logo dos provedores conhecidos, pelo id: github é o único id sem issuer, e google é o id que o DEPLOY.md usa.
  def icon
    case id
    when "github" then "github-logo"
    when "google" then "google-logo"
    end
  end
end
