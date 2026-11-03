# Grafo em json pronto para o canvas (graph_view_controller.js), o mesmo para o cofre inteiro e para o local da nota: o
# link de cada nota, o matiz da comunidade (a mesma conta do Explorer), o PageRank que dá o raio, a ligação inferida, que
# o canvas desenha tracejada, e a nota aberta, que fica em destaque.
module GraphDrawing
  extend ActiveSupport::Concern

  # Teto de nós que o canvas desenha; acima dele ficam os de maior PageRank, com a nota aberta, e total diz quantos eram.
  NODE_LIMIT = 2000

  included do
    rescue_from MarkuppApi::VaultUnavailable, with: :refuse_graph
  end

  private
    def render_graph(graph, current:)
      drawn = graph.capped(NODE_LIMIT, keep: current)
      render json: { nodes: drawn.nodes.map { graph_node(it, current) }, links: drawn.links.map { graph_link(it) }, total: graph.nodes.size }
    end

    def graph_node(node, current)
      { id: node.id, title: node.title, url: (graph_note_url(node) if node.id), hue: (helpers.community_hue(node.community) if node.community),
        rank: node.pagerank, unresolved: node.unresolved == true, current: node.id.present? && node.id == current }
    end

    # O link do nó que é nota: a página dela no painel. A leitura sem conta troca pelo link público.
    def graph_note_url(node) = note_path(node.id)

    def graph_link(link) = { source: link.source, target: link.target, inferred: link.type == "inferred" }

    # Cofre que a conta não lê mais: o canvas mostra a recusa no lugar do grafo.
    def refuse_graph(error)
      log_api_event("markupp_graph_forbidden", error, level: :warn)
      head :forbidden
    end
end
