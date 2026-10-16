class ApplicationController < ActionController::Base
  # Only allow modern browsers supporting webp images, web push, badges, import maps, CSS nesting, and CSS :has.
  allow_browser versions: :modern

  # Changes to the importmap will invalidate the etag for HTML responses
  stale_when_importmap_changes

  # Raiz de composição: o initializer injeta a fábrica do cliente da API; os testes trocam pela fake.
  class_attribute :markupp_api_factory

  rescue_from MarkuppApi::Error, with: :render_unreachable

  private
    def markupp_api = @markupp_api ||= markupp_api_factory.call

    # Cada controller tem a sua view unreachable: página inteira nas notas, aviso dentro da paleta na busca.
    def render_unreachable(error)
      Rails.logger.error({ event: "markupp_api_unreachable", message: error.message }.to_json)
      render :unreachable, status: :bad_gateway
    end
end
