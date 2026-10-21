# Linhas das visões planas do Explorer (notes/_explorer_groups): o número à direita de cada uma e a cor da comunidade.
module ExplorerHelper
  # Matizes das comunidades (--community-0 a 7, em tree.css), pelo id da comunidade: o grafo usa a mesma conta.
  COMMUNITY_HUES = 8

  def community_hue(community) = community % COMMUNITY_HUES

  # Estilo do grupo da comunidade, que pinta a marca e a borda; o das notas sem comunidade fica sem cor.
  def community_style(community) = ("--community: var(--community-#{community_hue(community)})" if community)

  # O grau nas mais ligadas e a idade nas mudadas por último; as outras visões não mostram número.
  def explorer_row_metric(note, mode)
    case mode
    when "linked" then note.degree.to_s
    when "recent" then tag.time(compact_age(note.updated_at), datetime: note.updated_at&.iso8601, title: note.updated_at && l(note.updated_at))
    end
  end
end
