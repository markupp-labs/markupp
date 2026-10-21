# Consentimento OAuth (ADR-0047): o servidor manda o navegador a /oauth/authorize/<id>, a conta logada aprova ou recusa,
# e o navegador volta ao cliente pelo endereço que a API devolve, e só por ele.
class OAuthAuthorizationsController < ApplicationController
  layout "public"

  rescue_from MarkuppApi::OAuthRequestNotFound, with: :render_gone_request

  # Cliente já autorizado com esses escopos volta direto, sem a tela.
  def show
    @oauth_request = OAuthRequest.find(credential.api, params[:id])
    decide("approve") if @oauth_request.already_authorized
  end

  def approve = decide("approve")

  def deny = decide("deny")

  private
    # O form vai fora do Turbo: o 303 leva o navegador ao cliente, que pode estar em outro host ou esquema.
    def decide(decision)
      redirect_to OAuthRequest.decide(credential.api, params[:id], decision), allow_other_host: true, status: :see_other
    end

    def render_gone_request(error)
      log_api_event("markupp_oauth_request_gone", error, request_id: params[:id])
      render :gone, status: :not_found
    end
end
