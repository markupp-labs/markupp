module Settings
  # Recusas de ligação do cofre ativo: a lista e o desfazer. Recusar fica na visão Links e no grafo
  # (::LinkRefusalsController). A recusa já desfeita volta como motivo no topo da lista recarregada.
  class LinkRefusalsController < VaultBaseController
    rescue_from MarkuppApi::LinkRefusalNotFound, with: :render_refusal
    helper_method :link_refusals

    def index
    end

    def destroy
      LinkRefusal.undo(credential.api, current_vault.id, params[:id])
      redirect_to settings_link_refusals_path, notice: t(".undone")
    end

    private
      def link_refusals = @link_refusals ||= LinkRefusal.all(credential.api, current_vault.id)

      def render_refusal(error)
        log_api_event("markupp_link_refusal_undo_refused", error, vault_id: current_vault.id, refusal_id: params[:id])
        @refusal = t("settings.link_refusals.errors.#{error.reason}")
        render :index, status: :not_found
      end
  end
end
