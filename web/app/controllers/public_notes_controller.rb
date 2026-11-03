# Cofre publicado em /p/:slug(/*path), para quem não tem conta: a árvore das notas, a página do cofre com o grafo e a
# nota pelo caminho sem .md, lida pelo mesmo renderizador do painel, com os wikilinks resolvidos pela lista pública.
class PublicNotesController < ApplicationController
  include PublicReading

  layout "published_vault"

  def show
    @vault = PublishedVault.find(public_reader, params[:slug])
    notes = @vault.notes(public_reader)
    @note = read_note(notes)
    return render_embed if request.format.json?

    @tree = Note::Tree.build(notes)
  end

  private
    # nil na página do cofre; caminho sem nota publicada levanta NotFound, como o slug sem cofre.
    def read_note(notes)
      path = requested_path or return
      listed = notes.find { it.location == "/#{path}" } or
        raise MarkuppApi::NotFound, "no published note at #{path.inspect} in #{params[:slug].inspect}, expected the path of a listed note without .md"
      @vault.note(public_reader, listed.id)
    end

    # O ![[nota]] da leitura pede o endereço da nota com .json (editor/note_embeds.js).
    def requested_path
      path = params[:path].presence
      path && request.format.json? ? path.delete_suffix(".json") : path
    end

    def render_embed = @note ? render(json: @note.attributes.slice("path", "content")) : head(:not_found)
end
