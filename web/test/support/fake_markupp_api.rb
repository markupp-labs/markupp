# Fica no lugar do MarkuppApi nos testes, sem rede.
class FakeMarkuppApi
  attr_reader :put_requests, :post_requests

  # conflict: true faz todo PUT sem force responder como se outro cliente tivesse salvo antes.
  def initialize(notes: [], unreachable: false, conflict: false)
    @notes = notes.map(&:stringify_keys)
    @unreachable = unreachable
    @conflict = conflict
    @put_requests = []
    @post_requests = []
  end

  def get(path)
    refuse_when_unreachable("GET", path)
    return @notes if path == "/notes"
    return search(URI(path).query) if path.start_with?("/notes/search?")

    note_at("GET", path)
  end

  def put(path, body)
    refuse_when_unreachable("PUT", path)
    @put_requests << body
    raise MarkuppApi::Conflict, "fake API simulates a conflict on PUT #{path}, expected force: true" if @conflict && !body[:force]

    note = note_at("PUT", path)
    refuse_path(body[:path], note)
    note.merge!(body.slice(:path, :content).stringify_keys)
  end

  # Como a API, recusa caminho já usado. O id segue a ordem dos POSTs: created-1, created-2...
  def post(path, body)
    refuse_when_unreachable("POST", path)
    raise ArgumentError, "fake API has no route for POST #{path}, expected /notes" unless path == "/notes"

    @post_requests << body
    raise MarkuppApi::DuplicatePath, "fake API already has a note at #{body[:path]}, expected a free path" if @notes.any? { it["path"] == body[:path] }

    @notes << body.slice(:path, :content).stringify_keys.merge("id" => "created-#{@post_requests.size}", "updated_at" => "2026-10-01T12:00:00.000Z")
    @notes.last
  end

  def delete(path)
    refuse_when_unreachable("DELETE", path)
    @notes.delete(note_at("DELETE", path))
    nil
  end

  private
    def refuse_when_unreachable(verb, path)
      raise MarkuppApi::Error, "fake API unreachable on #{verb} #{path}" if @unreachable
    end

    # Como a API: substring do conteúdo sem diferenciar caixa, até limit itens, sem o conteúdo.
    def search(query_string)
      params = URI.decode_www_form(query_string).to_h
      matches = @notes.select { |note| note["content"].to_s.downcase.include?(params.fetch("query").downcase) }
      matches.first(Integer(params.fetch("limit"))).map { |note| note.slice("id", "path", "updated_at") }
    end

    # Só a regra de caminho vazio do servidor; as outras (.., tamanho) ficam com ele.
    def refuse_path(new_path, note)
      raise MarkuppApi::InvalidPath, "fake API refuses path #{new_path.inspect}, expected a non blank path" if new_path.to_s.strip.empty?
      return unless @notes.any? { |other| other["path"] == new_path && !other.equal?(note) }

      raise MarkuppApi::DuplicatePath, "fake API already has a note at #{new_path.inspect}, expected a free path"
    end

    def note_at(verb, path)
      id = path.delete_prefix("/notes/")
      raise ArgumentError, "fake API has no route for #{verb} #{path}, expected /notes/{id}" if id == path

      @notes.find { |note| note["id"] == id } or raise MarkuppApi::NotFound, "fake API has no note #{id.inspect}, expected one of #{@notes.map { it["id"] }}"
    end
end
