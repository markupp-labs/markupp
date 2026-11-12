# Ligação, como a API lista: source liga a target; a não resolvida tem target nil e, se o indexador
# adivinhou a nota, inferred_target; type é explicit, embed, typed (com predicate), inferred ou, na ligação automática
# que o servidor cria, mentioned (cited_title é o título citado).
Link = Data.define(:source, :target, :cited_title, :type, :predicate, :inferred_target) do
  extend V1Listing

  # As ligações que a nota faz; type filtra pelo tipo, offset e limit paginam.
  def self.from_note(api, note_id, **query) = list_from(api, "notes", note_id, "links", **query)

  # As ligações resolvidas de outras notas para esta (backlinks).
  def self.to_note(api, note_id, **query) = list_from(api, "notes", note_id, "backlinks", **query)

  def self.from_api(attributes)
    new(source: Note.from_api(attributes["source"]), target: note_or_nil(attributes["target"]),
        inferred_target: note_or_nil(attributes["inferred_target"]), **attributes.slice("cited_title", "type", "predicate").symbolize_keys)
  end

  def self.note_or_nil(attributes) = attributes && Note.from_api(attributes)
  private_class_method :note_or_nil

  def unresolved? = target.nil?

  # Criada pelo servidor, nunca escrita na nota: é a que a recusa de ligação tira.
  def automatic? = Link::AUTOMATIC_TYPES.include?(type)
end

# Os tipos da ligação automática (ADR-0053); por enquanto só a por menção.
Link::AUTOMATIC_TYPES = %w[mentioned].freeze
