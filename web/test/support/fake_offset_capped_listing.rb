# Lista sob /v1 que, como a API, corta o offset no teto (maxOffset em markupp/internal/api/notes_handler.go) sem erro:
# depois do teto toda página vem cheia, a mesma. Levanta depois de max_calls chamadas, para o laço sem fim falhar o
# teste em vez de prendê-lo.
class FakeOffsetCappedListing
  attr_reader :requested_offsets

  def initialize(offset_cap:, max_calls:)
    @offset_cap = offset_cap
    @max_calls = max_calls
    @requested_offsets = []
  end

  def get(path)
    params = FakeMarkuppPaging.query(path)
    @requested_offsets << Integer(params.fetch("offset"))
    raise "fake listing got #{@requested_offsets.size} calls for #{path}, expected at most #{@max_calls}" if @requested_offsets.size > @max_calls

    full_page([ @requested_offsets.last, @offset_cap ].min, Integer(params.fetch("limit")))
  end

  private
    def full_page(offset, limit) = Array.new(limit) { { "id" => "n#{offset + it}", "path" => "n#{offset + it}.md" } }
end
