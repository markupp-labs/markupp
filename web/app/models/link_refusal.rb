# Recusa de ligação, como GET /v1/vaults/{id}/link-refusals devolve: o par de notas (na ordem do id) em que o servidor
# não cria ligação automática de tipo nenhum, nos dois sentidos, até alguém desfazer. O viewer só lê.
LinkRefusal = Data.define(:id, :notes, :created_at) do
  extend V1Listing

  # A mais nova primeiro.
  # shortcut: só a primeira página da API (100); paginar a lista quando um cofre passar disso.
  def self.all(api, vault_id) = list_from(api, "vaults", vault_id, "link-refusals", limit: Note::PAGE_LIMIT)

  # Recusar de novo o mesmo par devolve a recusa que existe.
  def self.refuse(api, vault_id, note_ids) = api.post(MarkuppApi.path("v1", "vaults", vault_id, "link-refusals"), note_ids:)

  def self.undo(api, vault_id, id) = api.delete(MarkuppApi.path("v1", "vaults", vault_id, "link-refusals", id))

  def self.from_api(attributes)
    new(id: attributes["id"], notes: attributes["notes"].map { Note.from_api(it) }, created_at: Time.zone.parse(attributes["created_at"]))
  end
end
