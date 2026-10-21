# Grafo local da nota aberta, para a visão Graph da barra lateral: a nota e as vizinhas a um ou dois saltos, em json.
class NoteGraphsController < ApplicationController
  include GraphDrawing

  # Os saltos que o seletor da visão oferece; a API iria até 3.
  DEPTHS = 1..2

  before_action :require_tenant

  def show
    note_id = params[:note_id]
    render_graph(Graph.around_note(credential.api, note_id, depth: params.fetch(:depth, 1).to_i.clamp(DEPTHS)), current: note_id)
  end
end
