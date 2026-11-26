# Bloquear e desbloquear IP ou faixa, na seção Acessos (ADR-0061). A recusa da API volta com o motivo.
module Settings
  class InstanceIpBlocksController < InstanceAdminController
    REFUSALS = [ MarkuppApi::InvalidIpBlock, MarkuppApi::BlocksOwnIp, MarkuppApi::IpBlockExists, MarkuppApi::IpBlockNotFound ].freeze
    rescue_from(*REFUSALS, with: :refuse)

    def create
      fields = params.expect(ip_block: [ :cidr, :note ])
      block = IpBlock.create(credential.api, cidr: fields[:cidr].to_s, note: fields[:note].to_s)
      redirect_to settings_instance_access_path, notice: t(".blocked", cidr: block.cidr)
    end

    def destroy
      IpBlock.destroy(credential.api, params[:id])
      redirect_to settings_instance_access_path, notice: t(".unblocked")
    end

    private
      def settings_section = "instance_access"

      def refuse(error)
        log_api_event("markupp_instance_ip_block_refused", error)
        redirect_to settings_instance_access_path, alert: t("settings.instance_access.errors.#{error.reason}")
      end
  end
end
