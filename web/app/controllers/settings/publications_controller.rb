module Settings
  # Seção Publication do cofre ativo (ADR-0048), só para owner e admin: publica com slug e título, mostra o link público
  # para copiar e despublica. O cofre ativo é o da primeira nota da árvore, como no Explorer e no grafo; os envios
  # trazem o cofre no campo escondido do form. A regra do slug fica na API, e a recusa volta no campo.
  # ponytail: seletor de cofre quando um tenant tiver mais de um.
  class PublicationsController < TenantAdminController
    SLUG_REFUSALS = { MarkuppApi::InvalidRequest => :unprocessable_content, MarkuppApi::SlugTaken => :conflict }.freeze

    rescue_from(*SLUG_REFUSALS.keys, with: :render_slug_refusal)

    def show
      @vault_id = load_note_tree.notes_within.first&.vault_id
      @publication = VaultPublication.find(credential.api, @vault_id) if @vault_id
    end

    def update
      @vault_id = params.expect(:vault_id)
      @slug, @title = params.expect(publication: [ :slug, :title ]).values_at(:slug, :title)
      publication = VaultPublication.publish(credential.api, @vault_id, slug: @slug, title: @title)
      redirect_to settings_publication_path, notice: t(".published", url: public_vault_url(publication.slug))
    end

    def destroy
      VaultPublication.unpublish(credential.api, params.expect(:vault_id))
      redirect_to settings_publication_path, notice: t(".unpublished")
    rescue MarkuppApi::NotPublished
      redirect_to settings_publication_path, alert: t(".not_published")
    end

    private
      def render_slug_refusal(error)
        log_api_event("markupp_publication_refused", error, vault_id: @vault_id, slug: @slug)
        @publication = VaultPublication.find(credential.api, @vault_id)
        @slug_error = t("settings.publications.errors.#{error.reason}", slug: @slug)
        render :show, status: SLUG_REFUSALS.fetch(error.class)
      end
  end
end
