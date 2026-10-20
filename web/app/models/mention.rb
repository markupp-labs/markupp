# Menção não ligada: o título (cited_title) de uma nota citado no texto de outra sem ligação entre elas, com a
# prior de o título apontar para a nota. É sugestão de ligação, nunca ligação.
Mention = Data.define(:note, :cited_title, :prior) do
  extend V1Listing

  # As notas cujo título esta nota cita sem ligar.
  def self.made_by(api, note_id, **query) = list_from(api, "notes", note_id, "mentions", **query)

  # As notas que citam um título desta sem ligar.
  def self.citing(api, note_id, **query) = list_from(api, "notes", note_id, "backmentions", **query)

  def self.from_api(attributes) = new(note: Note.from_api(attributes["note"]), cited_title: attributes["cited_title"], prior: attributes["prior"])
end
