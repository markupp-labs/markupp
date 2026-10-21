# Formulário de login. A regra de senha fica no servidor; o painel só traduz a recusa.
class SessionLogin
  include ActiveModel::Model
  include ActiveModel::Attributes

  attribute :email, :string
  attribute :password, :string

  # Devolve o par de tokens da API, ou nil com o motivo em :base. Campo vazio conta como credencial errada.
  def submit(public_api)
    public_api.post("/auth/login", email:, password:)
  rescue MarkuppApi::InvalidCredentials, MarkuppApi::InvalidRequest
    errors.add(:base, :invalid_credentials) && nil
  rescue MarkuppApi::LocalAuthDisabled
    errors.add(:base, :local_auth_disabled) && nil
  end
end
