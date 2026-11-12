# Leitura sem conta da nota pública (ADR-0048), fora do login: a API lê como o visitante, sem token, e o IP dele vai no
# X-Forwarded-For para o limite por IP da API. Endereço sem id público, id inexistente e nota privada caem no mesmo
# 404; o limite estourado, no 429. O slug do endereço que não é o do título atual redireciona de vez para o atual.
module PublicReading
  extend ActiveSupport::Concern

  included do
    skip_before_action :require_login
    rescue_from MarkuppApi::NotFound, with: :refuse_missing
    rescue_from MarkuppApi::RateLimited, with: :refuse_rate_limited
    layout "published"
  end

  private
    def public_reader = @public_reader ||= markupp_api_factory.call(forwarded_for: request.remote_ip)

    # A nota do endereço; com outro slug, nil depois de mandar ao endereço atual (o json do embed não redireciona).
    def find_public_note
      slug, public_id = PublicNote.id_in(params[:slug_and_id])
      @note = PublicNote.find(public_reader, public_id)
      redirect_to({ slug_and_id: @note.to_param }, status: :moved_permanently) if slug != @note.title_slug && request.format.html?
    end

    def refuse_missing(error)
      log_api_event("markupp_public_note_not_found", error, address: params[:slug_and_id])
      render_public_refusal("public_notes/not_found", :not_found)
    end

    def refuse_rate_limited(error)
      log_api_event("markupp_public_note_rate_limited", error, level: :warn, address: params[:slug_and_id])
      render_public_refusal("public_notes/rate_limited", :too_many_requests)
    end

    # Json (o grafo, o ![[nota]]) só com o status; a página com o motivo, sem a árvore.
    def render_public_refusal(template, status)
      @note = nil
      return head(status) if request.format.json?

      render template, status:, formats: :html
    end
end
