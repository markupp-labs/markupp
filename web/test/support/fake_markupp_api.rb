# Fica no lugar do MarkuppApi nos testes, sem rede.
class FakeMarkuppApi
  def initialize(notes: [], unreachable: false)
    @notes = notes.map(&:stringify_keys)
    @unreachable = unreachable
  end

  def get(path)
    raise MarkuppApi::Error, "fake API unreachable on GET #{path}" if @unreachable
    return @notes if path == "/notes"

    note_at(path)
  end

  private
    def note_at(path)
      id = path.delete_prefix("/notes/")
      raise ArgumentError, "fake API has no route for GET #{path}, expected /notes or /notes/{id}" if id == path

      @notes.find { |note| note["id"] == id } or raise MarkuppApi::NotFound, "fake API has no note #{id.inspect}, expected one of #{@notes.map { it["id"] }}"
    end
end
