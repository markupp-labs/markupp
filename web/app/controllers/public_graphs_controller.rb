# Grafo do cofre publicado em json, só leitura e sem conta: o mesmo canvas do painel (graph_view_controller.js), com o
# link público de cada nota. Fica em /p/:slug.json, que não colide com o caminho de nota nenhum.
class PublicGraphsController < ApplicationController
  include PublicReading
  include GraphDrawing

  def show
    render_graph(Graph.of_published_vault(public_reader, params[:slug]), current: nil)
  end

  private
    def graph_note_url(node) = helpers.published_note_path(params[:slug], Note.new(path: node.path))
end
