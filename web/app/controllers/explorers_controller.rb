# Conteúdo do Explorer sozinho, para o frame explorer_list da barra lateral: a visão escolhida (mode) fica no cookie
# explorer_mode, que as próximas páginas leem (SidebarHelper#current_explorer_mode), e a nota aberta (current) vem na
# query, para a linha dela ficar marcada.
class ExplorersController < ApplicationController
  before_action :require_tenant

  def show
    @mode = ExplorerListing.mode(params[:mode])
    cookies[:explorer_mode] = { value: @mode, expires: 1.year, same_site: :lax }
    @tree = load_note_tree
    @current = @tree.notes_within.find { it.id == params[:current] }
  end
end
