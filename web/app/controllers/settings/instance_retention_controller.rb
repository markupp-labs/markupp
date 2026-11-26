# Trocar o prazo de guarda do histórico e do IP, na seção Acessos (ADR-0061).
module Settings
  class InstanceRetentionController < InstanceAdminController
    rescue_from MarkuppApi::InvalidRetention, MarkuppApi::HostLocked, with: :refuse

    def update
      retention = InstanceRetention.update(credential.api, params.expect(retention: [ :days ])[:days].to_i)
      redirect_to settings_instance_access_path, notice: t(".saved", count: retention.days)
    end

    private
      def settings_section = "instance_access"

      def refuse(error)
        log_api_event("markupp_instance_retention_refused", error)
        redirect_to settings_instance_access_path, alert: t("settings.instance_access.errors.#{error.reason}")
      end
  end
end
