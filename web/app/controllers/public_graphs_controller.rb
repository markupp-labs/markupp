# Grafo da publicação da nota pública, só leitura e sem conta: o mesmo canvas do painel (graph_view_controller.js), com o
# link público de cada nota, que o slug da árvore completa.
class PublicGraphsController < ApplicationController
  include PublicReading
  include GraphDrawing

  before_action :find_public_note

  def show
    @tree = Note::Tree.build(@note.tree(public_reader))
    render_graph(@note.graph(public_reader), current: @note.public_id) if request.format.json?
  end

  private
    def graph_note_url(node)
      listed = @tree.notes_within.find { it.public_id == node.id }
      listed ? helpers.public_tree_note_path(listed) : public_note_path(node.id)
    end
end
