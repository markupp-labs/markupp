# Convites que a conta recebeu: o seletor de tenant carrega a lista num frame preguiçoso; aceitar ou recusar vem dele
# ou do onboarding.
class MyInvitesController < ApplicationController
  rescue_from MarkuppApi::InviteNotFound, MarkuppApi::AlreadyMember, with: :refuse_answer

  def index
    @invites = PendingInvite.mine(credential.api)
  end

  # Aceitar abre as notas do tenant novo, como criar um.
  def update
    tenant = PendingInvite.accept(credential.api, params[:id])
    credential.tenant_id = tenant.id
    redirect_to root_path, notice: t(".joined", name: tenant.name, role: t("tenants.roles.#{tenant.role}"))
  end

  def destroy
    PendingInvite.decline(credential.api, params[:id])
    redirect_back_or_to tenants_path, notice: t(".declined")
  end

  private
    def refuse_answer(error)
      Rails.logger.info({ event: "markupp_invite_refused", invite_id: params[:id], message: error.message }.to_json)
      redirect_back_or_to tenants_path, alert: t("my_invites.errors.#{error.reason}")
    end
end
