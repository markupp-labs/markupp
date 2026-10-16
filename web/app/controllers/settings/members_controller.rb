module Settings
  # Membros do tenant ativo: troca de papel e remoção. A recusa da API volta como motivo no topo da lista recarregada.
  class MembersController < BaseController
    REFUSAL_STATUS = { MarkuppApi::LastOwner => :conflict, MarkuppApi::RoleAboveOwn => :forbidden, MarkuppApi::MemberNotFound => :not_found }.freeze

    rescue_from(*REFUSAL_STATUS.keys, with: :render_refusal)
    helper_method :tenant_members

    def index
    end

    def update
      TenantMember.change_role(credential.api, params[:id], params.expect(member: [ :role ])[:role])
      redirect_to settings_members_path, notice: t(".updated")
    end

    def destroy
      TenantMember.remove(credential.api, params[:id])
      return leave_tenant if params[:id] == credential.account_id

      redirect_to settings_members_path, notice: t(".removed")
    end

    private
      def tenant_members = @tenant_members ||= TenantMember.all(credential.api)

      # Quem se remove perde o acesso: o painel esquece o tenant e manda escolher outro.
      def leave_tenant
        credential.tenant_id = nil
        redirect_to tenants_path, notice: t(".left", name: current_tenant.name)
      end

      def render_refusal(error)
        Rails.logger.info({ event: "markupp_member_change_refused", member_id: params[:id], message: error.message }.to_json)
        @refusal = t("settings.members.errors.#{error.class.name.demodulize.underscore}")
        render :index, status: REFUSAL_STATUS.fetch(error.class)
      end
  end
end
