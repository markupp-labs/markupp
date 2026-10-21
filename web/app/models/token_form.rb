# Formulário dos tokens que chegam por email: aceite de convite, troca de senha e confirmação de email.
# O painel só exige o token; a regra de senha fica no servidor, que o painel traduz para o campo.
class TokenForm
  include ActiveModel::Model
  include ActiveModel::Attributes

  attribute :token, :string
  attribute :password, :string

  validates :token, presence: true

  # Devolve a resposta da API ({} no 204), ou nil com o motivo no campo. Sem senha, o corpo leva só o token.
  def submit(api, path)
    return unless valid?

    api.post(path, { token:, password: }.compact) || {}
  rescue MarkuppApi::InvalidToken
    errors.add(:token, :invalid_token) && nil
  rescue MarkuppApi::InvalidRequest
    errors.add(:password, :invalid) && nil
  rescue MarkuppApi::AccountExists
    errors.add(:base, :account_exists) && nil
  rescue MarkuppApi::LocalAuthDisabled
    errors.add(:base, :local_auth_disabled) && nil
  rescue MarkuppApi::EmailUnavailable
    errors.add(:base, :email_unavailable) && nil
  end
end
