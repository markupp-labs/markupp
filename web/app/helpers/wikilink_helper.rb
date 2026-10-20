# [[wikilink]] na leitura: o note_mode_controller.js resolve o nome no navegador com este mapa de
# caminho sem .md para a url de cada nota, tirado da árvore que a barra lateral já carregou.
module WikilinkHelper
  def wikilink_targets(tree)
    tree.notes_within.to_h { |note| [ [ *note.folder_names, note.title ].join("/"), note_path(note) ] }
  end
end
