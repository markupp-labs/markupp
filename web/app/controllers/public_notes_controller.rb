# Nota pública em /p/{title_slug}-{public_id}, para quem não tem conta: a nota só em leitura, pelo mesmo renderizador do
# painel, com a árvore da publicação ao lado. Em json, o conteúdo para o ![[nota]] de outra nota pública.
class PublicNotesController < ApplicationController
  include PublicReading

  before_action :find_public_note

  def show
    return render(json: @note.to_h.slice(:title, :content)) if request.format.json?

    @tree = Note::Tree.build(@note.tree(public_reader))
  end
end
