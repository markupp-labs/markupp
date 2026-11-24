# Seção Sign-up do grupo Instance (ADR-0059): o administrador da instância abre e fecha o cadastro de cada método. Fechar
# não tira o login de quem já tem conta por ele.
module Settings
  class InstanceRegistrationController < InstanceAdminController
    rescue_from MarkuppApi::HostLocked, MarkuppApi::NotFound, with: :refuse

    def show
      @methods = RegistrationMethod.all(credential.api)
      @policy = AccountPolicy.fetch(credential.public_api)
    end

    def update
      choice = params.expect(registration: [ :method, :open ])
      RegistrationMethod.update(credential.api, choice[:method].to_s, open: choice[:open] == "true")
      redirect_to settings_instance_registration_path, notice: t(".saved")
    end

    private
      def refuse(error)
        log_api_event("markupp_instance_registration_refused", error)
        redirect_to settings_instance_registration_path, alert: t("settings.instance_registration.errors.#{error.reason}")
      end
  end
end
