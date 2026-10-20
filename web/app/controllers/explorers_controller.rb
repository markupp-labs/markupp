# Conteúdo do Explorer sozinho, para o frame explorer_list da barra lateral: a nota aberta (current) vem na query, para a
# linha dela ficar marcada.
class ExplorersController < ApplicationController
  before_action :require_tenant

  def show
    @tree = load_note_tree
    @current = @tree.notes_within.find { it.id == params[:current] }
  end
end
