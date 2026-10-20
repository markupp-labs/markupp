# Áreas ao lado das notas, como as configurações: o tenant ativo é opcional, a barra lateral desenha a árvore das notas
# e o voltar (botão e Esc) leva à página de onde a área foi aberta. Quem inclui diz o caminho da área em aside_area_path.
module AsideArea
  extend ActiveSupport::Concern

  included do
    before_action :pick_tenant, :remember_area_origin
    helper_method :sidebar_note_tree, :area_origin_path
  end

  private
    def pick_tenant
      @current_tenant = Tenant.pick(current_tenants, credential.tenant_id)
    end

    # Página de onde a área foi aberta, para o voltar. Andar dentro da área não muda a origem, e só vale caminho do
    # próprio painel (referer de outro site cai na raiz, sem redirecionar para fora).
    def remember_area_origin
      origin = same_site_path(request.referer)
      session[area_origin_key] = origin if origin && !origin.start_with?(aside_area_path)
    end

    def area_origin_path = session[area_origin_key] || root_path

    def area_origin_key = :"#{aside_area_path.delete_prefix("/")}_origin"

    def same_site_path(url)
      uri = URI.parse(url.to_s)
      return unless uri.host == request.host && uri.path.start_with?("/") && !uri.path.start_with?("//")

      [ uri.path, uri.query ].compact.join("?")
    rescue URI::InvalidURIError
      nil
    end

    # A mesma árvore das notas na barra lateral. Se só a listagem cair (API fora), a barra mostra um aviso curto e a
    # área segue; sessão vencida e tenant perdido continuam com o tratamento de sempre.
    def sidebar_note_tree
      load_note_tree
    rescue MarkuppApi::Error => error
      raise unless error.instance_of?(MarkuppApi::Error)

      log_api_event("markupp_note_tree_unreachable", error, level: :error)
      nil
    end
end
