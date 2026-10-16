module Settings
  # Convites do tenant ativo. Sem SMTP a API devolve o token, que aparece só nesta resposta: não vai para flash, log nem cache.
  class InvitesController < BaseController
    # Recusa da emissão: campo do formulário que mostra o motivo e status da resposta.
    FIELD_REFUSALS = { MarkuppApi::InvalidRequest => [ :email, :unprocessable_content ], MarkuppApi::AlreadyMember => [ :email, :conflict ],
                       MarkuppApi::RoleAboveOwn => [ :role, :forbidden ] }.freeze

    rescue_from(*FIELD_REFUSALS.keys, with: :render_field_refusal)
    helper_method :tenant_invites

    def index
      @role = "member"
    end

    def create
      @email, @role = params.expect(invite: [ :email, :role ]).values_at(:email, :role)
      @issued, @token = TenantInvite.issue(credential.api, email: @email, role: @role)
      return redirect_to(settings_invites_path, notice: t(".sent", email: @issued.email)) unless @token

      no_store
      @email = nil
      render :index, status: :created
    end

    def destroy
      TenantInvite.revoke(credential.api, params[:id])
      redirect_to settings_invites_path, notice: t(".revoked")
    rescue MarkuppApi::InviteNotFound
      redirect_to settings_invites_path, alert: t(".not_pending")
    end

    private
      def tenant_invites = @tenant_invites ||= TenantInvite.all(credential.api)

      def render_field_refusal(error)
        Rails.logger.info({ event: "markupp_invite_issue_refused", message: error.message }.to_json)
        field, status = FIELD_REFUSALS.fetch(error.class)
        @field_errors = { field => t("settings.invites.errors.#{error.class.name.demodulize.underscore}", email: @email) }
        render :index, status:
      end
  end
end
