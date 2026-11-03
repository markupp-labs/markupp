# Leitura sem conta do cofre publicado (ADR-0048), fora do login: a API lê como o visitante, sem token, e o IP dele vai
# no X-Forwarded-For para o limite por IP da API. Slug inexistente, cofre não publicado e nota que não é do cofre caem no
# mesmo 404; o limite estourado, no 429.
module PublicReading
  extend ActiveSupport::Concern

  included do
    skip_before_action :require_login
    rescue_from MarkuppApi::NotFound, with: :refuse_missing
    rescue_from MarkuppApi::RateLimited, with: :refuse_rate_limited
  end

  private
    def public_reader = @public_reader ||= markupp_api_factory.call(forwarded_for: request.remote_ip)

    def refuse_missing(error)
      log_api_event("markupp_public_vault_not_found", error, slug: params[:slug])
      render_public_refusal("public_notes/not_found", :not_found)
    end

    def refuse_rate_limited(error)
      log_api_event("markupp_public_vault_rate_limited", error, level: :warn, slug: params[:slug])
      render_public_refusal("public_notes/rate_limited", :too_many_requests)
    end

    # Json (o grafo, o ![[nota]]) só com o status; a página com o motivo no layout do cofre publicado.
    def render_public_refusal(template, status)
      return head(status) if request.format.json?

      render template, status:, formats: :html, layout: "published_vault"
    end
end
