# Próxima página de uma seção da visão Links: o Show more no fim de uma seção cheia pede a página que começa em offset,
# no frame dele (note_links/_more). A seção e o offset vêm antes do tenant: a falha da API também responde nesse frame.
class NoteLinkSectionsController < ApplicationController
  before_action :read_section_page
  before_action :require_tenant

  def show
    @items = NoteLinkSection.page(credential.api, params[:note_id], @section, offset: @offset)
  end

  private
    def read_section_page
      @section = params[:section]
      @offset = params[:offset].to_i
    end
end
