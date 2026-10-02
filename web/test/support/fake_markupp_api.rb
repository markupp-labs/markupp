# Fica no lugar do MarkuppApi nos testes, sem rede.
class FakeMarkuppApi
  attr_reader :put_requests

  # conflict: true faz todo PUT sem force responder como se outro cliente tivesse salvo antes.
  def initialize(notes: [], unreachable: false, conflict: false)
    @notes = notes.map(&:stringify_keys)
    @unreachable = unreachable
    @conflict = conflict
    @put_requests = []
  end

  def get(path)
    refuse_when_unreachable("GET", path)
    return @notes if path == "/notes"

    note_at("GET", path)
  end

  def put(path, body)
    refuse_when_unreachable("PUT", path)
    @put_requests << body
    raise MarkuppApi::Conflict, "fake API simulates a conflict on PUT #{path}, expected force: true" if @conflict && !body[:force]

    note_at("PUT", path).merge!(body.slice(:path, :content).stringify_keys)
  end

  private
    def refuse_when_unreachable(verb, path)
      raise MarkuppApi::Error, "fake API unreachable on #{verb} #{path}" if @unreachable
    end

    def note_at(verb, path)
      id = path.delete_prefix("/notes/")
      raise ArgumentError, "fake API has no route for #{verb} #{path}, expected /notes/{id}" if id == path

      @notes.find { |note| note["id"] == id } or raise MarkuppApi::NotFound, "fake API has no note #{id.inspect}, expected one of #{@notes.map { it["id"] }}"
    end
end
