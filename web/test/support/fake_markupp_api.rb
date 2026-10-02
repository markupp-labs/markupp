# Fica no lugar do MarkuppApi nos testes, sem rede.
class FakeMarkuppApi
  def initialize(notes: [], unreachable: false)
    @notes = notes
    @unreachable = unreachable
  end

  def get(path)
    raise MarkuppApi::Error, "fake API unreachable on GET #{path}" if @unreachable
    raise ArgumentError, "fake API has no route for GET #{path}, expected /notes" unless path == "/notes"

    @notes.map(&:stringify_keys)
  end
end
