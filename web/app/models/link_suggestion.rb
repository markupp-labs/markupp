# Nota que esta poderia ligar, pela vizinhança no grafo (resource_allocation) e pelo texto (cosine), fundidas em score.
LinkSuggestion = Data.define(:note, :score, :resource_allocation, :cosine) do
  extend V1Listing

  def self.for_note(api, note_id, **query) = list_from(api, "notes", note_id, "suggestions", **query)

  def self.from_api(attributes)
    new(note: Note.from_api(attributes["note"]), **attributes.slice("score", "resource_allocation", "cosine").symbolize_keys)
  end
end
