# Item do menu de contexto da árvore (notes/_tree_menu): clicar chama a ação de mesmo nome no note_tree_controller.js e
# fecha o menu. O atalho que faz o mesmo com o foco na linha vai no aria-keyshortcuts e aparece à direita, fora do nome
# acessível. note_only: o item some quando o menu é de uma pasta.
module NoteTreeHelper
  def tree_menu_item(name, keys, note_only: false, danger: false, **attributes)
    tag.button(type: "button", role: "menuitem", tabindex: -1, aria: { keyshortcuts: keys }, class: [ "tree-row context-menu-item", ("btn-danger" if danger) ],
               data: { note_only:, action: "note-tree##{name.to_s.camelize(:lower)} context-menu#close" }, **attributes) do
      safe_join([ t("notes.tree_menu.#{name}"), tag.kbd(shortcut_label(keys), aria: { hidden: true }) ])
    end
  end
end
