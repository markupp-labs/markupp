# Grafo para desenhar: nós (notas e títulos não resolvidos) e ligações como pares de índices em nodes.
Graph = Data.define(:nodes, :links) do
  # O cofre inteiro; orphans, unresolved e inferred false escondem, community recorta.
  def self.of_vault(api, vault_id, store: Rails.cache, **filters) = new(nodes: [], links: [])

  # As notas a até depth saltos da nota, com os mesmos filtros.
  def self.around_note(api, note_id, store: Rails.cache, **filters) = new(nodes: [], links: [])
end
