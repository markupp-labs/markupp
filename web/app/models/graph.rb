# Grafo para desenhar: nós (notas e títulos não resolvidos) e ligações como pares de índices em nodes.
# O corpo fica guardado em store com a ETag da API, por rota e filtros, e cada leitura revalida: no 304 vale o guardado.
# Guardar por rota não vaza entre contas: a API confere o acesso ao cofre antes de responder 304.
# Cada entrada vence em Graph::STORED_FOR, e o cache de produção tem teto de tamanho (config/environments/production.rb): as
# combinações de filtros que ninguém mais pede saem sozinhas.
Graph = Data.define(:nodes, :links) do
  # O cofre inteiro; orphans, unresolved e inferred false escondem, community recorta.
  def self.of_vault(api, vault_id, store: Rails.cache, **filters)
    revalidated(api, MarkuppApi.path("v1", "vaults", vault_id, "graph", **filters), store)
  end

  # As notas a até depth saltos da nota, com os mesmos filtros.
  def self.around_note(api, note_id, store: Rails.cache, **filters)
    revalidated(api, MarkuppApi.path("v1", "notes", note_id, "graph", **filters), store)
  end

  # O cofre publicado, lido sem conta (ADR-0048), sem filtros.
  def self.of_published_vault(api, slug, store: Rails.cache)
    revalidated(api, MarkuppApi.path("v1", "public", "vaults", slug, "graph"), store)
  end

  def self.revalidated(api, path, store)
    key = "panel/graph#{path}"
    kept = store.read(key)
    revision = api.conditional_get(path, etag: kept&.fetch("etag"))
    return from_api(kept.fetch("body")) if revision.nil?

    store.write(key, { "etag" => revision.etag, "body" => revision.body }, expires_in: Graph::STORED_FOR) if revision.etag
    from_api(revision.body)
  end

  def self.from_api(body) = new(nodes: body["nodes"].map { built(Graph::Node, it) }, links: body["links"].map { built(Graph::Link, it) })

  def self.built(type, attributes) = type.new(**type.members.to_h { [ it, attributes[it.to_s] ] })

  private_class_method :revalidated, :built

  # O grafo com no máximo limit nós, os de maior PageRank (e keep, o id de uma nota que fica sempre), na ordem de antes,
  # com as ligações entre eles.
  def capped(limit, keep: nil)
    return self if nodes.size <= limit

    new_index = kept_indexes(limit, keep).each_with_index.to_h
    kept_links = links.select { new_index.key?(it.source) && new_index.key?(it.target) }
    Graph.new(nodes: nodes.values_at(*new_index.keys), links: kept_links.map { it.with(source: new_index[it.source], target: new_index[it.target]) })
  end

  private
    # O índice da nota mantida e os dos mais centrais até o limite, em ordem crescente; nó sem PageRank vem por último.
    def kept_indexes(limit, keep)
      kept = nodes.index { keep && it.id == keep }
      ranked = nodes.each_index.reject { it == kept }.sort_by { [ -(nodes[it].pagerank || -1), it ] }
      [ kept, *ranked ].compact.first(limit).sort
    end
end

# Quanto tempo o corpo guardado vale antes de sair do cache, mesmo sem a API ter mudado de versão.
Graph::STORED_FOR = 6.hours

# Nó do grafo: nota, ou título citado sem nota (unresolved, com id e path nil).
Graph::Node = Data.define(:id, :title, :path, :degree, :pagerank, :community, :unresolved)

# Ligação entre dois nós, pelos índices em nodes.
Graph::Link = Data.define(:source, :target, :type, :predicate)
