# Publicações da FakeMarkuppApi (ADR-0048), como o contrato da API autenticada: a prévia, publicar, a lista do tenant,
# a indexação e despublicar, com as regras da API (nota por quem escreve, pasta e cofre por owner e admin, o nome do alvo
# conferido). A nota coberta ganha public_id na própria nota da FakeMarkuppNotes, como o gatilho do banco; a coberta por
# mais de uma pertence à mais ampla (cofre > pasta mais curta > nota).
#   publications: [{ kind: "folder", vault_id: "vault-1", folder_path: "docs", indexable: false }]
class FakeMarkuppPublications
  ROUTE = %r{\A/v1/(?:vaults/(?<vault_id>[^/?]+)/publications(?<preview>/preview)?|publications(?:/(?<id>[^/?]+))?)(?:\?|\z)}
  ADMIN_ROLES = %w[owner admin].freeze
  BREADTH = { "vault" => 0, "folder" => 1, "note" => 2 }.freeze
  PUBLISHED_AT = "2026-10-20T12:00:00Z".freeze

  # O id público que a fake dá à nota: 12 caracteres alfanuméricos, fixos por nota.
  def self.public_id_for(note_id) = Digest::MD5.hexdigest("public-#{note_id}")[0, 12]

  # Como a API: minúsculas, sem acento, o resto vira hífen, no máximo 60 caracteres; vazio vira nota.
  def self.title_slug(title) = I18n.transliterate(title.to_s).downcase.gsub(/[^a-z0-9]+/, "-").delete_prefix("-")[0, 60].delete_suffix("-").presence || "nota"

  def self.handles?(path) = ROUTE.match?(path)

  attr_reader :publications

  def initialize(publications, notes)
    @notes = notes
    @created = 0
    @publications = publications.map { stored(it.stringify_keys) }
    refresh_public_ids!
  end

  def get(path, role: nil, **)
    route = route_of("GET", path)
    params = FakeMarkuppPaging.query(path)
    return preview(route[:vault_id], params) if route[:preview]

    items = @publications.select { params["vault_id"].nil? || it["vault_id"] == params["vault_id"] }.map { view(it) }
    { "items" => items, "total" => items.size }
  end

  def post(path, body, role: nil, account: nil, **)
    vault_id = route_of("POST", path)[:vault_id]
    target = body[:target].stringify_keys
    refuse_role(target["kind"], role, "POST #{path}")
    publication = target_publication(vault_id, target)
    refuse_mismatch(body[:confirm], publication)
    publish(publication, body[:indexable], account)
  end

  def patch(path, body, **)
    publication = find(route_of("PATCH", path)[:id])
    publication["indexable"] = body[:indexable]
    view(publication)
  end

  def delete(path, role: nil, **)
    publication = find(route_of("DELETE", path)[:id])
    refuse_role(publication["kind"], role, "DELETE #{path}")
    @publications.delete(publication)
    refresh_public_ids!
    nil
  end

  # A publicação mais ampla que cobre a nota, ou nil: é ela que dá o recorte da leitura pública.
  def broadest_for(note) = @publications.select { covers?(it, note) }.min_by { [ BREADTH.fetch(it["kind"]), it["folder_path"].to_s.length ] }

  def notes_of(publication) = @notes.notes.select { covers?(publication, it) }

  def target_name(publication)
    case publication["kind"]
    when "note" then File.basename(note_by_id(publication["note_id"])["path"], ".md")
    when "folder" then publication["folder_path"]
    else FakeMarkuppVaults::DEFAULT_NAME
    end
  end

  private
    def route_of(verb, path)
      ROUTE.match(path) or raise ArgumentError, "fake API has no route for #{verb} #{path}, expected /v1/vaults/{id}/publications[/preview] or /v1/publications[/{id}]"
    end

    def stored(publication)
      @created += 1
      { "id" => "publication-#{@created}", "indexable" => false, "published_at" => PUBLISHED_AT,
        "published_by" => { "id" => "account-1", "name" => FakeMarkuppAccounts::DEFAULT_EMAIL } }.merge(publication)
    end

    def preview(vault_id, params)
      publication = target_publication(vault_id, params)
      { "kind" => publication["kind"], "target_name" => target_name(publication), "note_count" => notes_of(publication).size,
        "includes_future" => publication["kind"] != "note" }
    end

    # O alvo pedido como publicação ainda não gravada; nota que não existe e pasta sem nota respondem 404.
    def target_publication(vault_id, target)
      publication = { "vault_id" => vault_id, "kind" => target["kind"], "note_id" => target["note_id"], "folder_path" => target["path"] }.compact
      return publication if publication["kind"] == "vault" || notes_of(publication).any?

      raise MarkuppApi::NotFound, "fake API has nothing to publish at #{target.inspect}: #{target["kind"]}_not_found"
    end

    def refuse_role(kind, role, call)
      allowed = kind == "note" ? role != "viewer" : ADMIN_ROLES.include?(role)
      raise MarkuppApi::PublishForbidden, "fake API refuses #{call} of a #{kind} for role #{role.inspect}: publish_forbidden" unless allowed
    end

    def refuse_mismatch(confirm, publication)
      expected = target_name(publication)
      return if confirm == expected

      raise MarkuppApi::ConfirmationMismatch, "fake API refuses confirm #{confirm.inspect}: confirmation_mismatch, expected #{expected.inspect}"
    end

    def publish(publication, indexable, account)
      existing = @publications.find { it.slice("vault_id", "kind", "note_id", "folder_path") == publication }
      raise MarkuppApi::AlreadyPublished, "fake API already has #{existing["id"]}: already_published" if existing

      @publications << stored(publication.merge("indexable" => indexable == true, "published_by" => { "id" => account&.fetch("id"), "name" => account&.fetch("email") }))
      refresh_public_ids!
      view(@publications.last)
    end

    def find(id)
      @publications.find { it["id"] == id } or raise MarkuppApi::PublicationNotFound, "fake API has no publication #{id.inspect}: publication_not_found"
    end

    def covers?(publication, note)
      return false unless note["vault_id"] == publication["vault_id"]

      case publication["kind"]
      when "note" then note["id"] == publication["note_id"]
      when "folder" then note["path"].start_with?("#{publication["folder_path"]}/")
      else true
      end
    end

    # Como o gatilho do banco: nota coberta tem public_id, a descoberta perde.
    def refresh_public_ids!
      @notes.notes.each do |note|
        broadest_for(note) ? note["public_id"] = FakeMarkuppPublications.public_id_for(note["id"]) : note.delete("public_id")
      end
    end

    def note_by_id(id) = @notes.notes.find { it["id"] == id }

    def view(publication)
      note = note_by_id(publication["note_id"]) if publication["kind"] == "note"
      fields = { "vault_name" => FakeMarkuppVaults::DEFAULT_NAME, "target_name" => target_name(publication), "note_count" => notes_of(publication).size }
      fields.merge!("public_id" => note["public_id"], "title_slug" => FakeMarkuppPublications.title_slug(fields["target_name"])) if note
      publication.merge(fields)
    end
end
