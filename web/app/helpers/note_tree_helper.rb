# Item do menu de contexto da árvore (notes/_tree_menu): clicar chama a ação de mesmo nome no note_tree_controller.js e
# fecha o menu. O atalho que faz o mesmo com o foco na linha vai no aria-keyshortcuts e aparece à direita, fora do nome
# acessível, vindo do ShortcutsHelper pelo nome (shortcut, quando difere do item). note_only: o item some quando o menu é
# de uma pasta.
module NoteTreeHelper
  # Cada pedaço de 4 caracteres das guias ("│   ", "├── "...) vira uma célula que o CSS desenha da altura toda da linha
  # (.tree-cell): os caracteres sozinhos não alcançam a altura da linha e a guia ficava com vãos. O texto fica, invisível,
  # para copiar e para os testes.
  TREE_CELLS = { "│   " => "pipe", "    " => "blank", "├── " => "tee", "└── " => "elbow" }.freeze

  def tree_guide(branch)
    cells = branch.scan(/.{4}/).map { |chunk| tag.span(chunk, class: "tree-cell", data: { cell: TREE_CELLS.fetch(chunk) }) }
    tag.span(safe_join(cells), aria: { hidden: true }, class: "tree-guide")
  end

  def tree_menu_item(name, shortcut: name, note_only: false, danger: false, **attributes)
    keys = shortcut_key(shortcut)
    tag.button(type: "button", role: "menuitem", tabindex: -1, aria: { keyshortcuts: keys }, class: [ "tree-row context-menu-item", ("btn-danger" if danger) ],
               data: { note_only:, action: "note-tree##{name.to_s.camelize(:lower)} context-menu#close" }, **attributes) do
      safe_join([ t("notes.tree_menu.#{name}"), tag.kbd(shortcut_label(keys), aria: { hidden: true }) ])
    end
  end
end
