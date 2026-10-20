# Grafo local da nota aberta, para a visão Graph da barra lateral: a nota e as vizinhas, em json.
class NoteGraphsController < ApplicationController
  before_action :require_tenant

  def show
    render json: {}
  end
end
