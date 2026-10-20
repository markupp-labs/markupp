# Visões do Explorer sobre as notas do cofre, fora a árvore de pastas (folders, a Note::Tree): as mais centrais
# (PageRank), as mais ligadas (grau), as mudadas por último, as órfãs e todas agrupadas por comunidade.
class ExplorerListing
  # Notas de uma comunidade (community é o id dela) ou, com community nil, de uma lista sem grupo.
  Group = Data.define(:community, :notes)

  MODES = %w[folders central linked recent orphans communities].freeze

  # Visão conhecida, ou a árvore de pastas.
  def self.mode(_raw) = MODES.first

  def self.load(_api, _vault_id, _mode, page_limit: nil) = []
end
