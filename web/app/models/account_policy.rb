# Regras do cadastro que valem agora, como GET /v1/accounts/policy devolve (ADR-0059): se quem hospeda deixa cadastrar e
# se o email e senha cria conta nova. Os provedores dizem o próprio cadastro em LoginProvider.
AccountPolicy = Data.define(:registration_enabled, :password_registration_open) do
  # Servidor fora do ar mostra o formulário como antes: o cadastro, se fechado, recusa com a mensagem dele.
  def self.fetch(public_api)
    attributes = public_api.get(MarkuppApi.path("v1", "accounts", "policy"))
    new(registration_enabled: attributes.fetch("registration_enabled"), password_registration_open: attributes.fetch("password_registration_open"))
  rescue MarkuppApi::Error => error
    Rails.logger.warn({ event: "markupp_account_policy_unreachable", message: error.message }.to_json)
    new(registration_enabled: true, password_registration_open: true)
  end
end
