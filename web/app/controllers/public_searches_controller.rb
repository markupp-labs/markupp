# Busca da página pública, sem conta: a primeira página dos resultados, só nas notas da publicação da nota lida.
class PublicSearchesController < ApplicationController
  include PublicReading

  before_action :find_public_note

  def show
    @query = params[:q].to_s.strip
    @hits = @query.empty? ? [] : @note.search(public_reader, @query)
    @tree = Note::Tree.build(@note.tree(public_reader))
  end
end
