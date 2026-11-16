# Provedor externo de login que o servidor oferece (ADR-0054), como GET /v1/login-providers devolve.
LoginProvider = Data.define(:id, :name) do
  # Lista vazia quando o servidor não responde: o login por email e senha segue sem os botões.
  def self.all(public_api)
    public_api.get(MarkuppApi.path("v1", "login-providers")).map { new(id: it.fetch("id"), name: it.fetch("name")) }
  rescue MarkuppApi::Error => error
    Rails.logger.warn({ event: "markupp_login_providers_unreachable", message: error.message }.to_json)
    []
  end
end
