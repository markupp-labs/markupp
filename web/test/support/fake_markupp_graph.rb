# Rotas de grafo da FakeMarkuppApi sob /v1, com o cenário que o teste dá, pelo id da nota ou do cofre da rota:
#   { links: { "1" => [LinkResponse] }, backlinks:, mentions:, backmentions:, suggestions:,
#     communities: { "vault-1" => [CommunityResponse] }, graph: { "vault-1" => GraphResponse }, graph_etag: 'W/"1"' }
# O grafo filtrado vem do cenário com a query na chave ("vault-1?orphans=false"), senão o do id.
# Id sem cenário responde lista vazia e grafo sem nós. Sem graph_etag o grafo vem sem ETag, como antes do indexador.
class FakeMarkuppGraph
  ROUTE = %r{\A/v1/(?:notes|vaults)/(?<id>[^/?]+)/(?<resource>[a-z]+)(?:\?|\z)}
  EMPTY_GRAPH = { "nodes" => [], "links" => [] }.freeze

  # Os If-None-Match recebidos e a query de cada grafo pedido, na ordem.
  attr_reader :received_etags, :graph_queries

  def initialize(scenario)
    @scenario = scenario.deep_stringify_keys
    @received_etags = []
    @graph_queries = []
  end

  def get(path, **)
    route = ROUTE.match(path) or raise ArgumentError, "fake API has no route for GET #{path}, expected /v1/notes/{id}/... or /v1/vaults/{id}/..."
    params = FakeMarkuppPaging.query(path)
    return graph(route[:id], params) if route[:resource] == "graph"

    FakeMarkuppPaging.page(of_type(@scenario.dig(route[:resource], route[:id]) || [], params["type"]), params)
  end

  # Como a API: nil (304) quando o If-None-Match traz a versão atual do grafo; sem versão, sempre o corpo.
  def conditional_get(path, etag:, **)
    @received_etags << etag
    version = @scenario["graph_etag"]
    return if version && etag == version

    MarkuppApi::Revision.new(body: get(path), etag: version)
  end

  # Cenário: o indexador roda de novo e o grafo do id muda de versão.
  def change_graph!(id, graph, etag:)
    (@scenario["graph"] ||= {})[id] = graph.deep_stringify_keys
    @scenario["graph_etag"] = etag
  end

  private
    def graph(id, params)
      @graph_queries << params
      @scenario.dig("graph", "#{id}?#{params.to_query}") || @scenario.dig("graph", id) || EMPTY_GRAPH
    end

    def of_type(items, type) = type ? items.select { it["type"] == type } : items
end
