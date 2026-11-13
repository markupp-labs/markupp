# Seção Embedding do grupo Instance (ADR-0057): o administrador da instância escolhe o provedor de embedding da
# instalação inteira. Quem administra vem de /auth/me no login; a API confere de novo e recusa as outras contas.
module Settings
  class InstanceEmbeddingController < BaseController
    before_action :require_instance_admin
    rescue_from MarkuppApi::InvalidRequest, MarkuppApi::HostLocked, with: :render_refused

    def show
      @embedding = InstanceEmbedding.fetch(credential.api)
    end

    def update
      InstanceEmbedding.save(credential.api, **choice_params)
      redirect_to settings_instance_embedding_path, notice: t(".saved")
    end

    private
      def require_instance_admin
        redirect_to root_path, alert: t("markupp_api.errors.instance_admin_required") unless credential.instance_admin?
      end

      # A chave em branco fica como está; a caixa Remove apaga a guardada.
      def choice_params
        permitted = params.expect(embedding: [ :provider, :url, :model, :api_key, :clear_api_key ])
        api_key = permitted[:clear_api_key] == "1" ? "" : permitted[:api_key].presence
        { provider: permitted[:provider].to_s, url: permitted[:url].to_s, model: permitted[:model].to_s, api_key: }
      end

      # Volta ao formulário com o que foi digitado, menos a chave, e o motivo da recusa.
      def render_refused(error)
        log_api_event("markupp_instance_embedding_refused", error)
        @refusal = t("settings.instance_embedding.errors.#{error.reason}")
        @embedding = InstanceEmbedding.fetch(credential.api).with(**choice_params.except(:api_key))
        render :show, status: :unprocessable_entity
      end
  end
end
