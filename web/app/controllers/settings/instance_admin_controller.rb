# Seções da instância: só quem a administra entra. Quem administra vem de /auth/me a cada página; a API confere de novo e
# recusa as outras contas.
module Settings
  class InstanceAdminController < BaseController
    before_action :require_instance_admin

    private
      def require_instance_admin
        redirect_to root_path, alert: t("markupp_api.errors.instance_admin_required") unless instance_admin?
      end
  end
end
