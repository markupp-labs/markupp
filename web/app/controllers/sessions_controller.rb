class SessionsController < ApplicationController
  layout "public"

  skip_before_action :require_login
  limit_create_attempts to: 10, within: 3.minutes, redirect_path: :new_session_path

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
    end_session
    redirect_to new_session_path, notice: t(".logged_out")
  end
end
