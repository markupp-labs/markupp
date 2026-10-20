# Resultado da busca ranqueada: a nota resumida, o cofre, a relevância e o trecho com os termos casados entre
# os marcadores U+0002 e U+0003 (SearchSnippetHelper troca por <mark>).
SearchHit = Data.define(:note, :vault_id, :rank, :snippet) do
  # Do mais relevante ao menos; vault restringe a um cofre, offset e limit paginam.
  def self.search(api, query, **options) = []
end
