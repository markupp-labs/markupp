module Settings
  # Seção Publications do tenant ativo (ADR-0048): tudo que está público, do cofre escolhido no seletor (o ativo vem
  # marcado; all lista todos), com a indexação e o despublicar. A API decide quem muda o quê e só lista o que a conta vê;
  # o painel esconde os botões que ela recusaria (owner e admin mudam tudo, quem escreve só as notas).
  class PublicationsController < BaseController
    ALL_VAULTS = "all"

    skip_before_action :pick_tenant
    before_action :require_tenant
    helper_method :chosen_vault_id, :tenant_publications, :changes_publication?

    def index
    end

    def update
      publication = Publication.change_indexing(credential.api, params[:id], params[:indexable] == "true")
      name = publication.target_name
      redirect_to list_path(publication), notice: publication.indexable ? t(".indexed", name:) : t(".hidden", name:)
    rescue MarkuppApi::PublicationNotFound
      redirect_to list_path(nil), alert: t("settings.publications.gone")
    end

    # O nome e o cofre vêm da lista antes de apagar: o DELETE responde vazio.
    def destroy
      publication = Publication.all(credential.api).find { it.id == params[:id] }
      return redirect_to(list_path(nil), alert: t("settings.publications.gone")) unless publication

      Publication.unpublish(credential.api, publication.id)
      redirect_to list_path(publication), notice: t(".unpublished", name: publication.target_name)
    rescue MarkuppApi::PublicationNotFound
      redirect_to list_path(publication), alert: t("settings.publications.gone")
    end

    private
      # O cofre do seletor; sem escolha, o ativo. nil lista todos.
      def chosen_vault_id
        return if params[:vault_id] == ALL_VAULTS

        params[:vault_id].presence || current_vault&.id
      end

      def tenant_publications = @tenant_publications ||= Publication.all(credential.api, vault_id: chosen_vault_id)

      def changes_publication?(publication) = current_tenant.admin? || (publication.kind == "note" && current_tenant.writer?)

      # A lista de onde veio o pedido (o seletor manda o cofre no form); sem ele, a do cofre da publicação.
      def list_path(publication) = settings_publications_path(vault_id: params[:vault_id].presence || publication&.vault_id)
  end
end
