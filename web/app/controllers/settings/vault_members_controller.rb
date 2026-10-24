module Settings
  # Membros do cofre ativo: adicionar um colega do tenant, remover e sair (remover a si mesmo). A recusa da API volta
  # como motivo no topo da lista recarregada.
  class VaultMembersController < VaultBaseController
    REFUSAL_STATUS = { MarkuppApi::NotTenantMember => :unprocessable_content, MarkuppApi::AlreadyVaultMember => :conflict,
                       MarkuppApi::VaultMemberNotFound => :not_found }.freeze

    rescue_from(*REFUSAL_STATUS.keys, with: :render_refusal)
    helper_method :vault_members, :vault_candidates

    def index
    end

    def create
      VaultMember.add(credential.api, current_vault.id, params.expect(member: [ :user_id ])[:user_id])
      redirect_to settings_vault_members_path, notice: t(".added")
    end

    def destroy
      VaultMember.remove(credential.api, current_vault.id, params[:id])
      return leave_vault if params[:id] == credential.account_id

      redirect_to settings_vault_members_path, notice: t(".removed")
    end

    private
      def vault_members = @vault_members ||= VaultMember.all(credential.api, current_vault.id)

      # Só quem escreve adiciona: o viewer nem pede a lista de colegas.
      def vault_candidates = @vault_candidates ||= current_tenant.writer? ? VaultMember.candidates(credential.api, vault_members) : []

      # Quem sai perde o cofre: o painel o esquece e a próxima página ativa o primeiro que sobrar.
      def leave_vault
        credential.vault_id = nil
        redirect_to root_path, notice: t(".left", name: current_vault.name)
      end

      def render_refusal(error)
        log_api_event("markupp_vault_member_change_refused", error, vault_id: current_vault.id, member_id: params[:id] || params.dig(:member, :user_id))
        @refusal = t("settings.vault_members.errors.#{error.reason}")
        render :index, status: REFUSAL_STATUS.fetch(error.class)
      end
  end
end
