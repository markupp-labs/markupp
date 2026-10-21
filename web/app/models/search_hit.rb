# Resultado da busca ranqueada: a nota resumida, o cofre, a relevância e o trecho com os termos casados entre
# os marcadores U+0002 e U+0003 (SearchSnippetHelper troca por <mark>).
SearchHit = Data.define(:note, :vault_id, :rank, :snippet) do
  extend V1Listing

  # Do mais relevante ao menos; vault restringe a um cofre, offset e limit paginam.
  def self.search(api, query, **options)
    return [] if query.blank?

    list_from(api, "search", "notes", q: query, **options)
  end

  def self.from_api(attributes) = new(note: Note.from_api(attributes["note"]), **attributes.slice("vault_id", "rank", "snippet").symbolize_keys)
end
