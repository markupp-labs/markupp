# Leitura sem conta da FakeMarkuppApi pelo id público (ADR-0048): a nota, a árvore, a busca e o grafo recortados pela
# publicação mais ampla que cobre a nota (FakeMarkuppPublications). Id inexistente e nota não publicada respondem igual,
# 404 not_found; o limite por IP estourado, 429.
class FakeMarkuppPublicNotes
  ROUTE = %r{\A/v1/public/notes/(?<public_id>[^/?]+)(?<rest>/(?:tree|search|graph))?(?:\?|\z)}
  WIKILINK = /\[\[([^\[\]|#\n]+)/
  GRAPH_ETAG = 'W/"public-1"'.freeze

  def self.handles?(path) = ROUTE.match?(path)

  def initialize(publications, notes)
    @publications = publications
    @notes = notes
  end

  # Cenário: o IP passou do limite da leitura pública.
  def limit_public_reads! = @rate_limited = true

  def get(path, **)
    route = ROUTE.match(path) or raise ArgumentError, "fake API has no route for GET #{path}, expected /v1/public/notes/{public_id}[/tree|/search|/graph]"
    note = public_note(route[:public_id], path)
    params = FakeMarkuppPaging.query(path)
    case route[:rest]
    when "/tree" then listing(tree_items(note), params)
    when "/search" then listing(search_hits(note, params.fetch("q")), params)
    else reading(note)
    end
  end

  # Como a API: nil (304) com a versão atual; o grafo da publicação com os nós pelo id público.
  def conditional_get(path, etag:, **)
    note = public_note(ROUTE.match(path)[:public_id], path)
    return if etag == GRAPH_ETAG

    nodes = siblings(note).map { summary(it).merge("degree" => 0) }
    MarkuppApi::Revision.new(body: { "nodes" => nodes, "links" => [] }, etag: GRAPH_ETAG)
  end

  private
    def public_note(public_id, path)
      raise MarkuppApi::RateLimited, "fake API limits GET #{path}: rate_limited, expected fewer reads per minute" if @rate_limited

      @notes.notes.find { it["public_id"] == public_id } or raise MarkuppApi::NotFound, "fake API has no public note #{public_id.inspect}: not_found"
    end

    def siblings(note) = @publications.notes_of(@publications.broadest_for(note)).sort_by { it["path"] }

    def title_of(note) = File.basename(note["path"], ".md")

    def summary(note) = { "public_id" => note["public_id"], "title" => title_of(note), "title_slug" => FakeMarkuppPublications.title_slug(title_of(note)) }

    def listing(items, params) = { "items" => FakeMarkuppPaging.page(items, params), "total" => items.size }

    # O caminho relativo à pasta publicada; na nota e no cofre, o caminho inteiro.
    def tree_items(note)
      folder = @publications.broadest_for(note)["folder_path"]
      siblings(note).map { summary(it).merge("path" => folder ? it["path"].delete_prefix("#{folder}/") : it["path"]) }
    end

    def search_hits(note, query)
      siblings(note).select { it["content"].to_s.downcase.include?(query.downcase) }
                    .map { summary(it).merge("snippet" => it["content"].to_s.gsub(/#{Regexp.escape(query)}/i) { "\u0002#{it}\u0003" }, "rank" => 1.0) }
    end

    def reading(note)
      publication = @publications.broadest_for(note)
      summary(note).merge("content" => note["content"], "updated_at" => note["updated_at"], "indexable" => publication["indexable"],
                          "publication" => { "kind" => publication["kind"], "title" => publication_title(publication),
                                             "root_public_id" => (note["public_id"] if publication["kind"] == "note") },
                          "links" => links_of(note))
    end

    def publication_title(publication)
      publication["kind"] == "folder" ? publication["folder_path"].split("/").last : @publications.target_name(publication)
    end

    # Cada [[nome]] da nota, com o id público do alvo quando ele é público.
    def links_of(note)
      note["content"].to_s.scan(WIKILINK).flatten.map(&:strip).uniq.map do |cited|
        target = @notes.notes.find { it["path"].delete_suffix(".md") == cited || title_of(it) == cited }
        public_target = target if target&.key?("public_id")
        { "cited_title" => cited, "kind" => "wikilink", "public_id" => public_target&.fetch("public_id"),
          "title_slug" => (FakeMarkuppPublications.title_slug(title_of(public_target)) if public_target) }
      end
    end
end
