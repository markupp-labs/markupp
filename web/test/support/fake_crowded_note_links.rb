# Cenário de grafo da FakeMarkuppApi para uma nota com mais ligações do que cabem numa página da API: count itens em cada
# seção da visão Links. As notas de cada seção são "<kind> 001", "<kind> 002"..., na ordem em que a API as devolve.
class FakeCrowdedNoteLinks
  def initialize(note, count:)
    @note = note
    @count = count
  end

  def graph
    id = @note[:id]
    { backlinks: { id => notes("backlink").map { link(it, @note) } },
      links: { id => notes("outgoing").map { link(@note, it) } },
      backmentions: { id => notes("mention").map { { note: it, cited_title: @note[:title], prior: 0.5 } } },
      suggestions: { id => notes("suggestion").map { { note: it, score: 0.01, resource_allocation: 0, cosine: 0 } } } }
  end

  private
    def notes(kind)
      (1..@count).map { format("%s %03d", kind, it) }.map { { id: it.tr(" ", "-"), title: it, path: "#{it}.md" } }
    end

    def link(source, target)
      { source:, target:, cited_title: target[:title], type: "explicit", predicate: nil, inferred_target: nil }
    end
end
