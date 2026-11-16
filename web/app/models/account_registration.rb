# Formulário de criar conta. A regra de senha e de quem pode se cadastrar fica no servidor; o painel só traduz a recusa.
class AccountRegistration
  include ActiveModel::Model
  include ActiveModel::Attributes
  include NewPassword

  attribute :email, :string
  attribute :password, :string

  # Devolve o par de tokens da conta nova, ou nil com o motivo no campo.
  def submit(public_api)
    return unless valid?

    public_api.post(MarkuppApi.path("v1", "accounts"), email:, password:)
  rescue MarkuppApi::InvalidRequest => e
    refuse_password(e)
  rescue MarkuppApi::AccountExists
    errors.add(:base, :account_exists) && nil
  rescue MarkuppApi::RegistrationDisabled
    errors.add(:base, :registration_disabled) && nil
  rescue MarkuppApi::LocalAuthDisabled
    errors.add(:base, :local_auth_disabled) && nil
  end
end
