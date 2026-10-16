class SessionsController < ApplicationController
  layout "public"

  skip_before_action :require_login
  rate_limit to: 10, within: 3.minutes, only: :create,
             with: -> { redirect_to new_session_path, alert: t("sessions.create.rate_limited") }

  def new
    @login = SessionLogin.new
  end

  def create
    @login = SessionLogin.new(params.expect(session: [ :email, :password ]))
    pair = @login.submit(credential.public_api)
    return render(:new, status: :unprocessable_content) unless pair

    redirect_to start_session(pair)
  end

  def destroy
    revoke_refresh_token
    credential.sign_out
    redirect_to new_session_path, notice: t(".logged_out")
  end

  private
    # A API responde 204 sempre; fora do ar, o painel sai do mesmo jeito e o refresh vence sozinho.
    def revoke_refresh_token
      credential.public_api.post("/auth/logout", refresh_token: credential.refresh_token)
    rescue MarkuppApi::Error => error
      Rails.logger.warn({ event: "markupp_logout_not_revoked", message: error.message }.to_json)
    end
end
