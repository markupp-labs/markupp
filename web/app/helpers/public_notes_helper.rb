# Links da leitura sem conta (/p/{title_slug}-{public_id}).
module PublicNotesHelper
  # O endereço público de uma nota da árvore pública (Note com public_id e title_slug).
  def public_tree_note_path(note) = public_note_path("#{note.title_slug}-#{note.public_id}")

  # O mapa dos [[wikilinks]] (WikilinkHelper) só com as ligações a notas públicas: as outras ficam como texto.
  def public_wikilink_targets(note) = note.public_links.to_h { [ it.cited_title, public_note_path("#{it.title_slug}-#{it.public_id}") ] }

  # O começo da publicação, pelo slug que a árvore conhece; sem ele, só o id, e o endereço redireciona ao atual.
  # Pasta e cofre não têm nota de entrada (root_public_id nulo): devolve nil e o título fica sem link.
  def public_root_path(note, tree)
    root_id = note.publication.root_public_id
    return if root_id.nil?

    root = tree&.notes_within&.find { it.public_id == root_id }
    root ? public_tree_note_path(root) : public_note_path(root_id)
  end
end
