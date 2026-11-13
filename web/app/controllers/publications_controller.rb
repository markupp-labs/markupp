# Publicar no cofre ativo (ADR-0048): a nota (pelo cabeçalho dela), a pasta (pelo menu da árvore) ou o cofre inteiro
# (pela General dele). O new desenha, no frame do diálogo do layout, o que fica público pela prévia da API e pede o nome
# do alvo; o create manda o nome digitado, que a API confere de novo, e volta para a página de onde veio.
class PublicationsController < ApplicationController
  REFUSALS = [ MarkuppApi::ConfirmationMismatch, MarkuppApi::AlreadyPublished, MarkuppApi::NotFound ].freeze

  before_action :require_tenant, :require_vault
  rescue_from MarkuppApi::InvalidRequest, with: :refuse_target
  rescue_from(*REFUSALS, with: :refuse_publication)

  def new
    @target = publication_target
    @preview = Publication.preview(credential.api, current_vault.id, @target)
  end

  def create
    publication = Publication.publish(credential.api, current_vault.id, publication_target, confirm: params[:confirm].to_s, indexable: params[:indexable] == "1")
    redirect_back_or_to root_path, notice: published_notice(publication)
  end

  private
    def publication_target = Publication::Target.from(kind: params[:kind], note_id: params[:note_id].presence, path: params[:path].presence)

    # A nota tem endereço próprio; pasta e cofre dizem quantas notas abriram.
    def published_notice(publication)
      return t("publications.create.published_note", url: public_note_url(publication.public_param)) if publication.public_param

      t("publications.create.published", count: publication.note_count)
    end

    def refuse_target(error)
      log_api_event("markupp_publication_target_refused", error, kind: params[:kind])
      head :bad_request
    end

    # O nome a digitar vem da prévia, e não do que foi digitado.
    def refuse_publication(error)
      log_api_event("markupp_publication_refused", error, kind: params[:kind])
      name = Publication.preview(credential.api, current_vault.id, publication_target).target_name if error.is_a?(MarkuppApi::ConfirmationMismatch)
      redirect_back_or_to root_path, alert: t("publications.refused.#{error.reason}", name:)
    end
end
