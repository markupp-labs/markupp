# Seção Login externo do grupo Instance: o administrador da instância lista, cria, edita, liga, desliga e apaga os
# provedores de login externo (GitHub, Google ou outro OIDC). O segredo nunca volta à página.
module Settings
  class InstanceLoginProvidersController < InstanceAdminController
    rescue_from MarkuppApi::InvalidRequest, MarkuppApi::HostLocked, MarkuppApi::LoginProviderExists,
                MarkuppApi::LoginProviderIssuerLocked, with: :render_refused
    rescue_from MarkuppApi::LoginProviderNotFound, with: :refuse_missing

    def index
      @listing = InstanceLoginProvider.fetch(credential.api)
      @draft = InstanceLoginProvider.draft(preset: params[:preset])
    end

    def create
      InstanceLoginProvider.create(credential.api, **provider_params)
      redirect_to settings_instance_login_providers_path, notice: t(".created")
    end

    def update
      InstanceLoginProvider.update(credential.api, params[:id], **provider_params)
      redirect_to settings_instance_login_providers_path, notice: t(".saved")
    end

    def destroy
      removed = InstanceLoginProvider.destroy(credential.api, params[:id])
      redirect_to settings_instance_login_providers_path, notice: t(".deleted", count: removed)
    end

    private
      # O segredo em branco fica de fora: na edição mantém o guardado.
      def provider_params
        permitted = params.expect(login_provider: [ :id, :name, :issuer, :client_id, :client_secret, :enabled ])
        { id: permitted[:id].to_s, name: permitted[:name].to_s, issuer: permitted[:issuer].to_s, client_id: permitted[:client_id].to_s,
          client_secret: permitted[:client_secret].presence, enabled: permitted[:enabled] == "1" }.compact
      end

      # Volta à lista com o que foi digitado, menos o segredo, e o motivo da recusa.
      def render_refused(error)
        log_api_event("markupp_instance_login_provider_refused", error)
        @refusal = t("settings.instance_login_providers.errors.#{error.reason}")
        @listing = InstanceLoginProvider.fetch(credential.api)
        @draft = InstanceLoginProvider.draft(**(action_name == "create" ? provider_params : {}))
        @edited_id = params[:id] if action_name == "update"
        @listing = @listing.with(providers: @listing.providers.map { keep_typed(it) }) if @edited_id
        render :index, status: :unprocessable_entity
      end

      def keep_typed(provider)
        return provider unless provider.id == @edited_id

        provider.with(**provider_params.slice(:name, :issuer, :client_id, :enabled))
      end

      def refuse_missing(error)
        log_api_event("markupp_instance_login_provider_refused", error)
        redirect_to settings_instance_login_providers_path, alert: t("settings.instance_login_providers.errors.#{error.reason}")
      end
  end
end
