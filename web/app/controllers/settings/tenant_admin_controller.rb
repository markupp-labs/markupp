# Seções do tenant ativo: só owner e admin entram; o papel vem de /me/tenants a cada página.
module Settings
  class TenantAdminController < BaseController
    skip_before_action :pick_tenant
    before_action :require_tenant
    before_action :require_admin

    private
      def require_admin
        redirect_to root_path, alert: t("markupp_api.errors.forbidden") unless current_tenant.admin?
      end
  end
end
