# Seção Contas do grupo Instance (ADR-0061): o administrador da instância lista e busca as contas e, em cada uma, vê as
# sessões e os apps, bloqueia, desbloqueia e apaga. A API recusa bloquear ou apagar a própria conta.
module Settings
  class InstanceAccountsController < InstanceAdminController
    rescue_from MarkuppApi::AccountNotFound, with: :refuse_missing
    rescue_from MarkuppApi::CannotTargetSelf, with: :refuse_self

    def index
      @offset = params[:offset].to_i.clamp(0, V1Listing::MAX_OFFSET)
      @accounts = InstanceAccount.page(credential.api, q: params[:q], status: params[:status], offset: @offset)
    end

    def show
      @detail = InstanceAccount.find(credential.api, params[:id])
      @sessions = InstanceAccount.sessions(credential.api, params[:id])
      @authorizations = InstanceAccount.authorizations(credential.api, params[:id])
    end

    def update
      blocked = params.expect(account: [ :blocked ])[:blocked] == "1"
      InstanceAccount.set_blocked(credential.api, params[:id], blocked)
      redirect_to settings_instance_account_path(params[:id]), notice: blocked ? t(".blocked") : t(".unblocked")
    end

    def destroy
      InstanceAccount.destroy(credential.api, params[:id])
      redirect_to settings_instance_accounts_path, notice: t(".deleted")
    end

    private
      def refuse_missing(error)
        log_api_event("markupp_instance_account_refused", error)
        redirect_to settings_instance_accounts_path, alert: t("settings.instance_accounts.errors.account_not_found")
      end

      def refuse_self(error)
        log_api_event("markupp_instance_account_refused", error)
        redirect_to settings_instance_account_path(params[:id]), alert: t("settings.instance_accounts.errors.cannot_target_self")
      end
  end
end
