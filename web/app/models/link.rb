# Ligação, como a API lista: source liga a target; a não resolvida tem target nil e, se o indexador
# adivinhou a nota, inferred_target; type é explicit, embed, typed (com predicate) ou inferred.
Link = Data.define(:source, :target, :cited_title, :type, :predicate, :inferred_target) do
  # As ligações que a nota faz; type filtra pelo tipo, offset e limit paginam.
  def self.from_note(api, note_id, **query) = []

  # As ligações resolvidas de outras notas para esta (backlinks).
  def self.to_note(api, note_id, **query) = []
end
