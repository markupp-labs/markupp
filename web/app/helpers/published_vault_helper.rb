# Links da leitura sem conta do cofre publicado (/p/:slug): a nota pelo caminho sem .md depois do slug.
module PublishedVaultHelper
  def published_note_path(slug, note) = public_vault_path(slug, path: note.location.delete_prefix("/"))

  # O mapa dos [[wikilinks]] (WikilinkHelper) com o link público de cada nota da árvore.
  def published_wikilink_targets(slug, tree) = wikilink_targets(tree, url_of: ->(note) { published_note_path(slug, note) })
end
