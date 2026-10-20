# Grafo do cofre inteiro em tela cheia, como o graph view do Obsidian: a página desenha o canvas, que pede os nós e as
# ligações em json. O cofre é o da nota de onde a tela foi aberta (note) ou, sem ela, o da primeira nota da árvore; o
# Esc volta a essa nota, ou ao começo.
class GraphsController < ApplicationController
  include GraphDrawing

  before_action :require_tenant

  def show
    respond_to do |format|
      format.html { load_page }
      format.json { render_graph(Graph.of_vault(credential.api, params.expect(:vault)), current: params[:note]) }
    end
  end

  private
    def load_page
      @tree = load_note_tree
      @origin = @tree.notes_within.find { it.id == params[:note] }
      @vault_id = (@origin || @tree.notes_within.first)&.vault_id
    end
end
