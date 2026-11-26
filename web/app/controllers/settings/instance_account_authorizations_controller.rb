# Revogar um app autorizado de outra conta, na página dela na seção Contas (ADR-0061).
module Settings
  class InstanceAccountAuthorizationsController < InstanceAdminController
    def destroy
      InstanceAccount.revoke_authorization(credential.api, params[:instance_account_id], params[:id])
      redirect_to settings_instance_account_path(params[:instance_account_id]), notice: t("settings.authorizations.destroy.revoked")
    rescue MarkuppApi::AuthorizationNotFound
      redirect_to settings_instance_account_path(params[:instance_account_id]), alert: t("settings.authorizations.destroy.not_found")
    end

    private
      def settings_section = "instance_accounts"
  end
end
