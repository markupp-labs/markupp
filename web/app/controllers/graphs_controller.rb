# Grafo do cofre inteiro em tela cheia, como o graph view do Obsidian: a página desenha o canvas, que pede os nós e as
# ligações em json. O cofre é o da nota de onde a tela foi aberta (note) ou, sem ela, o da primeira nota da árvore; o
# Esc volta a essa nota, ou ao começo. Os filtros vêm na query da página e do json, com os nomes da API: orphans,
# unresolved e inferred false escondem, community recorta; valor que a API não aceita fica de fora.
class GraphsController < ApplicationController
  include GraphDrawing

  HIDING_FILTERS = %i[orphans unresolved inferred].freeze
  # Comunidades que o seletor oferece, as maiores primeiro: uma página da API.
  # ponytail: cofre com mais comunidades que isso não lista as menores; paginar o seletor se aparecer.
  COMMUNITY_CHOICES = 100

  before_action :require_tenant

  def show
    respond_to do |format|
      format.html { load_page }
      format.json { render_graph(Graph.of_vault(credential.api, params.expect(:vault), **graph_filters), current: params[:note]) }
    end
  end

  private
    def load_page
      @tree = load_note_tree
      @origin = @tree.notes_within.find { it.id == params[:note] }
      @vault_id = (@origin || @tree.notes_within.first)&.vault_id
      @filters = graph_filters
      @communities = vault_communities
    end

    def graph_filters
      hidden = HIDING_FILTERS.select { params[it] == "false" }.index_with(false)
      community = Integer(params[:community], exception: false)
      community ? hidden.merge(community:) : hidden
    end

    # Cofre recusado: o seletor fica só com "All communities" e o canvas mostra a recusa.
    def vault_communities
      return [] unless @vault_id

      Community.in_vault(credential.api, @vault_id, limit: COMMUNITY_CHOICES)
    rescue MarkuppApi::VaultUnavailable => error
      log_api_event("markupp_graph_forbidden", error, level: :warn)
      []
    end
end
