# Rotas de notas da FakeMarkuppApi: listagem, busca, conflito e caminho duplicado, como a API.
class FakeMarkuppNotes
  # Cofre de toda nota que o cenário não põe em outro.
  VAULT_ID = "vault-1"

  attr_reader :notes, :put_requests, :post_requests

  # conflict: true faz todo PUT sem force responder como se outro cliente tivesse salvo antes.
  def initialize(notes:, conflict:)
    @notes = notes.map { { "vault_id" => VAULT_ID }.merge(it.stringify_keys) }
    @conflict = conflict
    @put_requests = []
    @post_requests = []
  end

  def get(path, **)
    return @notes if path == "/notes"

    note_at("GET", path)
  end

  def put(path, body, role: nil, **)
    refuse_viewer(role, "PUT #{path}")
    @put_requests << body
    raise MarkuppApi::Conflict, "fake API simulates a conflict on PUT #{path}, expected force: true" if @conflict && !body[:force]

    note = note_at("PUT", path)
    refuse_path(body[:path], note)
    note.merge!(body.slice(:path, :content).stringify_keys)
  end

  # Como a API, recusa caminho já usado. O id segue a ordem dos POSTs: created-1, created-2... A rota sem prefixo grava
  # no cofre padrão; a de /v1, no cofre da rota.
  def post(path, body, role: nil, **)
    vault_id = path == "/notes" ? VAULT_ID : FakeMarkuppNoteIndex::VAULT_NOTES.match(path)&.[](:vault_id)
    raise ArgumentError, "fake API has no route for POST #{path}, expected /notes or /v1/vaults/{id}/notes" unless vault_id

    refuse_viewer(role, "POST #{path}")
    @post_requests << body
    refuse_path(body[:path], nil)

    @notes << body.slice(:path, :content).stringify_keys.merge("vault_id" => vault_id, "id" => "created-#{@post_requests.size}", "updated_at" => "2026-10-01T12:00:00.000Z")
    @notes.last
  end

  def delete(path, role: nil, **)
    refuse_viewer(role, "DELETE #{path}")
    @notes.delete(note_at("DELETE", path))
    nil
  end

  private
    # Como o requireWriter da API: viewer só lê.
    def refuse_viewer(role, call)
      raise MarkuppApi::ReadOnly, "fake API refuses #{call} for role viewer: read_only, expected owner, admin or member" if role == "viewer"
    end

    # Como a API: substring do conteúdo sem diferenciar caixa, até limit itens, sem o conteúdo.
    # Caminho vazio ou com .. como o servidor; o limite de tamanho fica com ele.
    def refuse_path(new_path, note)
      if new_path.to_s.strip.empty? || new_path.include?("..")
        raise MarkuppApi::InvalidPath, "fake API refuses path #{new_path.inspect}, expected a non blank path without .."
      end
      return unless @notes.any? { |other| other["path"] == new_path && !other.equal?(note) }

      raise MarkuppApi::DuplicatePath, "fake API already has a note at #{new_path.inspect}, expected a free path"
    end

    def note_at(verb, path)
      id = path.delete_prefix("/notes/")
      raise ArgumentError, "fake API has no route for #{verb} #{path}, expected /notes/{id}" if id == path

      @notes.find { |note| note["id"] == id } or raise MarkuppApi::NotFound, "fake API has no note #{id.inspect}, expected one of #{@notes.map { it["id"] }}"
    end
end
