# [[wikilink]] na leitura: o note_mode_controller.js resolve o nome no navegador com este mapa de
# caminho sem .md para a url de cada nota, tirado da árvore que a barra lateral já carregou. url_of dá a url da nota:
# a do painel por padrão, a pública na leitura sem conta.
module WikilinkHelper
  def wikilink_targets(tree, url_of: method(:note_path))
    tree.notes_within.to_h { |note| [ [ *note.folder_names, note.title ].join("/"), url_of.call(note) ] }
  end
end
