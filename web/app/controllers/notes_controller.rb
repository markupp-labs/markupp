class NotesController < ApplicationController
  rescue_from MarkuppApi::Error, with: :render_unreachable

  def index
    @tree = Note::Tree.build(Note.all)
  end

  private
    def render_unreachable(error)
      Rails.logger.error({ event: "markupp_api_unreachable", message: error.message }.to_json)
      render :unreachable, status: :bad_gateway
    end
end
