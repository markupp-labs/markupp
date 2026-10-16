# Rotas de email da conta na FakeMarkuppApi: esqueci a senha, troca por token, pedido e confirmação de verificação.
# Cada token emitido vai para sent_tokens, a caixa de entrada da fake, de onde o teste tira o token como a pessoa tiraria do email.
class FakeMarkuppAccountEmail
  PUBLIC_CALLS = [ "POST /auth/password/forgot", "POST /auth/password/reset" ].freeze

  # [{ kind: :password_reset | :email_verification, email:, token:, used: }], na ordem de envio.
  attr_reader :sent_tokens

  def initialize(accounts, auth, email_enabled:)
    @accounts = accounts
    @auth = auth
    @email_enabled = email_enabled
    @sent_tokens = []
  end

  def public?(call) = PUBLIC_CALLS.include?(call)

  def post(path, body, account:, **)
    case path
    when "/auth/password/forgot" then forgot_password(body)
    when "/auth/password/reset" then reset_password(body)
    when "/auth/email/verification" then request_verification(account)
    when "/auth/email/verify" then confirm_email(body, account)
    else raise ArgumentError, "fake API has no route for POST #{path}, expected /auth/password/forgot, /auth/password/reset, /auth/email/verification or /auth/email/verify"
    end
  end

  private
    # Como a API: 202 exista a conta ou não; só a conta que existe recebe o token.
    def forgot_password(body)
      check_password_flow("POST /auth/password/forgot")
      raise MarkuppApi::InvalidRequest, "fake API refuses POST /auth/password/forgot with #{body.inspect}, expected an email" if body[:email].blank?

      account = @accounts.find_by_email(body[:email])
      send_token(:password_reset, account) if account
      nil
    end

    # Como a API: a senha é conferida antes do token, e o token vale uma vez.
    def reset_password(body)
      check_password_flow("POST /auth/password/reset")
      raise MarkuppApi::InvalidRequest, "fake API refuses password of #{body[:password].to_s.bytesize} bytes, expected 1 to #{FakeMarkuppAuth::PASSWORD_LIMIT}" unless valid_password?(body[:password])

      account = consume_token(:password_reset, body[:token], "POST /auth/password/reset")
      account["password"] = body[:password]
      nil
    end

    def request_verification(account)
      raise MarkuppApi::EmailUnavailable, "fake API has no email on POST /auth/email/verification: email_unavailable" unless @email_enabled
      raise_already_verified(account, "POST /auth/email/verification")

      send_token(:email_verification, account)
      nil
    end

    # Como a API: o token só serve à conta logada que o pediu.
    def confirm_email(body, account)
      raise_already_verified(account, "POST /auth/email/verify")

      consume_token(:email_verification, body[:token], "POST /auth/email/verify", account:)
      account["verified"] = true
      nil
    end

    def check_password_flow(call)
      raise MarkuppApi::EmailUnavailable, "fake API has no email on #{call}: email_unavailable" unless @email_enabled
      raise MarkuppApi::LocalAuthDisabled, "fake API has local auth disabled on #{call}" unless @auth.local_auth?
    end

    def raise_already_verified(account, call)
      return unless account.fetch("verified", true)

      raise MarkuppApi::EmailAlreadyVerified, "fake API refuses #{call} for #{account["email"]}: email_already_verified"
    end

    def valid_password?(password) = password.to_s.strip.present? && password.bytesize <= FakeMarkuppAuth::PASSWORD_LIMIT

    # Tokens seguem a ordem de envio: password_reset-token-1, email_verification-token-2...
    def send_token(kind, account)
      @sent_tokens << { kind:, email: account["email"], token: "#{kind}-token-#{@sent_tokens.size + 1}", used: false }
    end

    def consume_token(kind, token, call, account: nil)
      sent = @sent_tokens.find { !it[:used] && it[:kind] == kind && it[:token] == token && (account.nil? || it[:email] == account["email"]) }
      raise MarkuppApi::InvalidToken, "fake API refuses #{kind} token #{token.inspect} on #{call}: invalid_token, expected an unused one sent by email" unless sent

      sent[:used] = true
      @accounts.find_by_email(sent[:email])
    end
end
