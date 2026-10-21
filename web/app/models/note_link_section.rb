# Seção da visão Links (note_links/show): de onde vem a lista de cada uma, uma página por vez. A página tem 100 itens, o
# máximo da API.
module NoteLinkSection
  PAGE_SIZE = 100
  SOURCES = {
    "backlinks" => ->(api, note_id, **query) { Link.to_note(api, note_id, **query) },
    "outgoing" => ->(api, note_id, **query) { Link.from_note(api, note_id, **query) },
    "mentions" => ->(api, note_id, **query) { Mention.citing(api, note_id, **query) },
    "suggestions" => ->(api, note_id, **query) { LinkSuggestion.for_note(api, note_id, **query) }
  }.freeze
  NAMES = SOURCES.keys.freeze

  # A página da seção que começa em offset.
  def self.page(api, note_id, name, offset: 0) = SOURCES.fetch(name).call(api, note_id, offset:, limit: PAGE_SIZE)
end
