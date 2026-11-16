# Recusas de ligação da FakeMarkuppApi sob /v1/vaults/{id}/link-refusals, como a API: por cofre, da mais nova para a
# mais antiga, só quem não é viewer recusa e desfaz, o par não tem sentido e recusar de novo devolve a recusa que existe.
# refusals: [{ id:, vault_id:, note_ids: [a, b], created_at: }], da mais nova para a mais antiga.
class FakeMarkuppLinkRefusals
  ROUTE = %r{\A/v1/vaults/(?<vault_id>[^/?]+)/link-refusals(?:/(?<id>[^/?]+))?(?:\?|\z)}
  CREATED_AT = "2026-11-01T12:00:00Z"

  def initialize(notes, refusals)
    @notes = notes
    @refusals = refusals.map { { "vault_id" => FakeMarkuppNotes::VAULT_ID, "created_at" => CREATED_AT }.merge(it.deep_stringify_keys) }
    @created = 0
  end

  # O par recusado, em qualquer sentido: a API tira a ligação automática dele das listas na hora.
  def refused?(note_id, other_id) = @refusals.any? { it["note_ids"].sort == [ note_id, other_id ].sort }

  def get(path, **)
    vault_id = route_of("GET", path)[:vault_id]
    FakeMarkuppPaging.page(in_vault(vault_id).map { view(it) }, FakeMarkuppPaging.query(path))
  end

  def post(path, body, role:, **)
    call = "POST #{path}"
    refuse_viewer(role, call)
    vault_id = route_of("POST", path)[:vault_id]
    note_ids = valid_pair(body[:note_ids], vault_id, call)
    view(in_vault(vault_id).find { it["note_ids"].sort == note_ids.sort } || add(vault_id, note_ids))
  end

  def delete(path, role:, **)
    call = "DELETE #{path}"
    refuse_viewer(role, call)
    route = route_of("DELETE", path)
    refusal = in_vault(route[:vault_id]).find { it["id"] == route[:id] } or
      raise MarkuppApi::LinkRefusalNotFound, "fake API refuses #{call}: link_refusal_not_found, expected one of #{@refusals.map { it["id"] }}"
    @refusals.delete(refusal)
    nil
  end

  private
    def route_of(verb, path)
      route = ROUTE.match(path)
      return route if route && route[:id].nil? == (verb != "DELETE")

      raise ArgumentError, "fake API has no route for #{verb} #{path}, expected /v1/vaults/{id}/link-refusals[/{refusal_id}]"
    end

    def refuse_viewer(role, call)
      raise MarkuppApi::ReadOnly, "fake API refuses #{call} for role viewer: read_only, expected owner, admin or member" if role == "viewer"
    end

    def in_vault(vault_id) = @refusals.select { it["vault_id"] == vault_id }

    def note(id) = @notes.notes.find { it["id"] == id }

    def valid_pair(note_ids, vault_id, call)
      raise MarkuppApi::InvalidRequest, "fake API refuses #{call} with note_ids #{note_ids.inspect}: invalid_request, expected two ids" unless note_ids.is_a?(Array) && note_ids.size == 2
      raise MarkuppApi::SameNote, "fake API refuses #{call} with note_ids #{note_ids.inspect}: same_note, expected two different notes" if note_ids.uniq.size == 1
      return note_ids if note_ids.all? { note(it)&.fetch("vault_id") == vault_id }

      raise MarkuppApi::NoteOutsideVault, "fake API refuses #{call} with note_ids #{note_ids.inspect}: note_outside_vault, expected notes of #{vault_id}"
    end

    def add(vault_id, note_ids)
      @created += 1
      @refusals.unshift({ "id" => "created-refusal-#{@created}", "vault_id" => vault_id, "note_ids" => note_ids, "created_at" => CREATED_AT }).first
    end

    def view(refusal)
      { "id" => refusal["id"], "notes" => refusal["note_ids"].sort.map { summary(note(it)) }, "created_at" => refusal["created_at"] }
    end

    def summary(note) = { "id" => note["id"], "title" => File.basename(note["path"], ".md"), "path" => note["path"] }
end
