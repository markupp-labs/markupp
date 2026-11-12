# Recusar ligação no cofre ativo, pela visão Links e pelo grafo: a ligação automática entre as duas notas sai das
# listas na hora e não volta até alguém desfazer a recusa nas configurações do cofre. Pela visão Links (html, no frame
# note_links) o frame da nota aberta recarrega com o aviso no status; pelo grafo (json) o aviso vem no corpo. A recusa
# da API volta com o motivo do mesmo jeito.
class LinkRefusalsController < ApplicationController
  REFUSAL_STATUS = { MarkuppApi::InvalidRequest => :bad_request, MarkuppApi::SameNote => :bad_request,
                     MarkuppApi::NoteOutsideVault => :unprocessable_content,
                     MarkuppApi::ReadOnly => :forbidden, MarkuppApi::VaultUnavailable => :forbidden }.freeze

  before_action :require_tenant, :require_vault
  rescue_from(*REFUSAL_STATUS.keys, with: :answer_refusal)

  def create
    LinkRefusal.refuse(credential.api, current_vault.id, params.expect(note_ids: []))
    answer(t(".refused"), status: :created)
  end

  private
    def answer(message, status:)
      respond_to do |format|
        format.html { redirect_to note_links_path(params.expect(:note_id)), flash: { links_status: message } }
        format.json { render json: { message: }, status: }
      end
    end

    def answer_refusal(error)
      log_api_event("markupp_link_refusal_refused", error, vault_id: current_vault&.id, note_ids: params[:note_ids])
      answer(t("link_refusals.errors.#{error.reason}"), status: REFUSAL_STATUS.fetch(error.class))
    end
end
