# Senha que alguém escolhe: a confirmação tem de repetir a senha, e a recusa da API vira o motivo no campo. A regra de
# força fica no servidor; sem senha no formulário (password_confirmation nil) não há o que confirmar.
module NewPassword
  extend ActiveSupport::Concern

  REFUSALS = { MarkuppApi::PasswordTooShort => :too_short, MarkuppApi::CommonPassword => :common }.freeze

  included do
    attribute :password_confirmation, :string
    validates :password, confirmation: true
  end

  private
    def refuse_password(error) = errors.add(:password, REFUSALS.fetch(error.class, :invalid)) && nil
end
