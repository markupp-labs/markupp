# Visões do Explorer sobre as notas do cofre, fora a árvore de pastas (folders, a Note::Tree): as mais centrais
# (PageRank), as mais ligadas (grau), as mudadas por último, as órfãs e todas agrupadas por comunidade.
class ExplorerListing
  # Notas de uma comunidade (community é o id dela) ou, com community nil, de uma lista sem grupo ou das notas sem
  # comunidade.
  Group = Data.define(:community, :notes)

  MODES = %w[folders central linked recent orphans communities].freeze
  TOP_LIMIT = 50
  # O máximo que a API devolve numa página.
  PAGE_LIMIT = 100
  TOP_QUERIES = { "central" => { sort: "pagerank" }, "linked" => { sort: "degree" }, "recent" => { sort: "updated" } }.freeze

  # Visão conhecida, ou a árvore de pastas.
  def self.mode(raw) = MODES.include?(raw) ? raw : MODES.first

  # As visões de ranking mostram só o topo; órfãs e comunidades, todas as notas.
  def self.load(api, vault_id, mode, page_limit: PAGE_LIMIT)
    case mode
    when "orphans" then [ Group.new(nil, every_note(api, vault_id, page_limit, orphan: true)) ]
    when "communities" then by_community(every_note(api, vault_id, page_limit, sort: "pagerank"))
    else [ Group.new(nil, Note.in_vault(api, vault_id, **TOP_QUERIES.fetch(mode), limit: TOP_LIMIT)) ]
    end
  end

  def self.every_note(api, vault_id, page_limit, **query) = Note.list_all_from(api, "vaults", vault_id, "notes", page_limit:, **query)

  # Das maiores para as menores, empate pelo id; dentro de cada uma a ordem que veio (PageRank). Sem comunidade no fim.
  def self.by_community(notes)
    grouped = notes.group_by(&:community)
    loose = grouped.delete(nil)
    groups = grouped.sort_by { |id, members| [ -members.size, id ] }.map { |id, members| Group.new(id, members) }
    loose ? groups << Group.new(nil, loose) : groups
  end

  private_class_method :every_note, :by_community
end
