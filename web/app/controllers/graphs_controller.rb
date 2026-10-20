# Grafo do cofre inteiro em tela cheia, como o graph view do Obsidian: a página desenha o canvas, que pede os nós e as
# ligações em json.
class GraphsController < ApplicationController
  before_action :require_tenant

  def show
    respond_to do |format|
      format.html
      format.json { render json: {} }
    end
  end
end
