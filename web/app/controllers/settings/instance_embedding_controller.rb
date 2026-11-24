# Seção Embedding do grupo Instance (ADR-0057): o administrador da instância escolhe o provedor de embedding da
# instalação inteira.
module Settings
  class InstanceEmbeddingController < InstanceAdminController
    rescue_from MarkuppApi::InvalidRequest, MarkuppApi::HostLocked, MarkuppApi::ApiKeyRequired, with: :render_refused

    def show
      @embedding = InstanceEmbedding.fetch(credential.api)
    end

    def update
      InstanceEmbedding.save(credential.api, **choice_params)
      redirect_to settings_instance_embedding_path, notice: t(".saved")
    end

    private
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
