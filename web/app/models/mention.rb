# Menção não ligada: o título (cited_title) de uma nota citado no texto de outra sem ligação entre elas, com a
# prior de o título apontar para a nota. É sugestão de ligação, nunca ligação.
Mention = Data.define(:note, :cited_title, :prior) do
  # As notas cujo título esta nota cita sem ligar.
  def self.made_by(api, note_id, **query) = []

  # As notas que citam um título desta sem ligar.
  def self.citing(api, note_id, **query) = []
end
