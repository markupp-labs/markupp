# Encerrar uma sessão de outra conta, na página dela na seção Contas (ADR-0061).
module Settings
  class InstanceAccountSessionsController < InstanceAdminController
    def destroy
      InstanceAccount.end_session(credential.api, params[:instance_account_id], params[:id])
      redirect_to settings_instance_account_path(params[:instance_account_id]), notice: t("settings.sessions.destroy.ended")
    rescue MarkuppApi::SessionNotFound
      redirect_to settings_instance_account_path(params[:instance_account_id]), alert: t("settings.sessions.destroy.not_found")
    end

    private
      def settings_section = "instance_accounts"
  end
end
