# Publicação de cofres na FakeMarkuppApi (ADR-0048): a do cofre, sob /v1/vaults/{id}/publication, só para owner e admin
# escreverem, e a leitura sem conta sob /v1/public/vaults/{slug}, sobre as notas da FakeMarkuppNotes e o grafo da
# FakeMarkuppGraph do cofre publicado. Slug inexistente e cofre não publicado respondem igual, 404.
#   publications: [{ vault_id: "vault-1", slug: "docs", title: "Docs" }]
class FakeMarkuppPublications
  PUBLICATION = %r{\A/v1/vaults/(?<vault_id>[^/?]+)/publication\z}
  PUBLIC = %r{\A/v1/public/vaults/(?<slug>[^/?]+)(?<rest>/[^?]*)?(?:\?|\z)}
  SLUG_FORMAT = /\A[a-z0-9-]{3,63}\z/
  ADMIN_ROLES = %w[owner admin].freeze
  PUBLISHED_AT = "2026-10-20T12:00:00Z".freeze

  def initialize(publications, notes, graph)
    @publications = publications.map { it.stringify_keys.reverse_merge("published_at" => PUBLISHED_AT, "published_by" => "account-1") }
    @notes = notes
    @graph = graph
  end

  def self.handles?(path) = PUBLICATION.match?(path) || PUBLIC.match?(path)

  def self.public?(path) = PUBLIC.match?(path)

  # Cenário: o IP passou do limite da leitura pública.
  def limit_public_reads! = @rate_limited = true

  def publications = @publications

  def get(path, role: nil, **)
    return publication_of(PUBLICATION.match(path)[:vault_id]) if PUBLICATION.match?(path)

    public_read(path)
  end

  # Como a API: no 304 responde nil; o grafo público tem a versão do cenário do grafo.
  def conditional_get(path, etag:, **)
    @graph.conditional_get("/v1/vaults/#{published(path)['vault_id']}/graph", etag:)
  end

  def put(path, body, role: nil, **)
    vault_id = admin_route(path, role, "PUT")
    slug, title = body.values_at(:slug, :title)
    refuse_slug(slug, vault_id)
    existing = @publications.find { it["vault_id"] == vault_id }
    return existing.merge!("slug" => slug, "title" => title.presence || "Vault") if existing

    @publications << { "vault_id" => vault_id, "slug" => slug, "title" => title.presence || "Vault", "published_at" => PUBLISHED_AT, "published_by" => "account-1" }
    @publications.last
  end

  def delete(path, role: nil, **)
    vault_id = admin_route(path, role, "DELETE")
    publication_of(vault_id)
    @publications.reject! { it["vault_id"] == vault_id }
    nil
  end

  private
    def publication_of(vault_id)
      @publications.find { it["vault_id"] == vault_id } or
        raise MarkuppApi::NotPublished, "fake API has no publication for vault #{vault_id.inspect}: not_published, expected one of #{@publications.map { it['vault_id'] }}"
    end

    def admin_route(path, role, verb)
      route = PUBLICATION.match(path) or raise ArgumentError, "fake API has no route for #{verb} #{path}, expected #{PUBLICATION.source}"
      raise MarkuppApi::Forbidden, "fake API refuses #{verb} #{path} for role #{role.inspect}: forbidden, expected owner or admin" unless ADMIN_ROLES.include?(role)

      route[:vault_id]
    end

    def refuse_slug(slug, vault_id)
      raise MarkuppApi::InvalidRequest, "fake API refuses slug #{slug.inspect}: invalid_request, expected #{SLUG_FORMAT.source}" unless SLUG_FORMAT.match?(slug.to_s)
      return unless @publications.any? { it["slug"] == slug && it["vault_id"] != vault_id }

      raise MarkuppApi::SlugTaken, "fake API already has slug #{slug.inspect}: slug_taken, expected a free slug"
    end

    def public_read(path)
      publication = published(path)
      rest = PUBLIC.match(path)[:rest].to_s
      return { "slug" => publication["slug"], "title" => publication["title"], "published_at" => publication["published_at"], "graph_version" => 1 } if rest.empty?
      return FakeMarkuppPaging.page(vault_notes(publication).sort_by { it["path"] }.map { list_item(it) }, FakeMarkuppPaging.query(path)) if rest == "/notes"

      public_note(publication, rest.delete_prefix("/notes/"))
    end

    def published(path)
      raise MarkuppApi::RateLimited, "fake API limits GET #{path}: rate_limited, expected fewer reads per minute" if @rate_limited

      slug = PUBLIC.match(path)[:slug]
      @publications.find { it["slug"] == slug } or raise MarkuppApi::NotFound, "fake API has no published vault #{slug.inspect}: not_found"
    end

    def vault_notes(publication) = @notes.notes.select { it["vault_id"] == publication["vault_id"] }

    def public_note(publication, id)
      note = vault_notes(publication).find { it["id"] == id } or raise MarkuppApi::NotFound, "fake API has no public note #{id.inspect}: not_found"
      list_item(note).merge("content" => note["content"])
    end

    def list_item(note) = { "id" => note["id"], "title" => File.basename(note["path"], ".md"), "path" => note["path"], "updated_at" => note["updated_at"] }
end
