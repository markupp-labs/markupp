import { EditorView } from "@codemirror/view"
import { Prec } from "@codemirror/state"

// Modo Vim do editor, como o VSCodeVim: o @replit/codemirror-vim só baixa com o modo ligado (sem preload no
// config/importmap.rb). Os comandos Ex do markupp viram eventos no editor (vim:write, vim:quit, vim:open, vim:mode),
// que o formulário da nota trata; o editor não sabe salvar nem navegar.
// Sair pelo teclado: Esc vai ao modo normal, e um Esc já no normal deixa o próximo Tab ou Shift+Tab sair do editor,
// pelo mesmo tab-focus do CodeMirror que o Esc sem Vim usa (e pelo mesmo prazo). Fora do insert o Tab não escreve.
const TAB_RELEASE_MS = 2000

// [nome, abreviação, o que fazer com os argumentos]. :e sem caminho e :edit trocam para o modo de edição.
const EX_COMMANDS = [
  [ "write", "w", () => [ "vim:write" ] ],
  [ "wq", "wq", () => [ "vim:write", { quit: true } ] ],
  [ "quit", "q", (args) => [ "vim:quit", { force: args == "!" } ] ],
  [ "edit", "e", (args) => (args ? [ "vim:open", { path: args } ] : [ "vim:mode", { mode: "edit" } ]) ],
  [ "read", "read", () => [ "vim:mode", { mode: "read" } ] ],
  [ "split", "split", () => [ "vim:mode", { mode: "split" } ] ],
]

let library

// Extensões do modo Vim; a primeira chamada baixa a biblioteca e registra os comandos Ex (o registro do Vim é global).
export async function vimModeExtensions() {
  library ??= import("@replit/codemirror-vim").then((vimLibrary) => defineExCommands(vimLibrary))
  const { vim, getCM } = await library
  return [
    tabLeavesAfterNormalEscape(getCM), vim({ status: true }), vimTheme,
    EditorView.editorAttributes.of({ "data-vim-keys": "" }),
  ]
}

function defineExCommands(vimLibrary) {
  EX_COMMANDS.forEach(([ name, prefix, toEvent ]) => {
    vimLibrary.Vim.defineEx(name, prefix, (cm, params) => {
      const [ type, detail ] = toEvent(params.argString?.trim() ?? "")
      cm.cm6.dom.dispatchEvent(new CustomEvent(type, { bubbles: true, detail }))
    })
  })
  return vimLibrary
}

// Antes do Vim: o Esc no modo normal libera o Tab; fora do insert o Tab e o Shift+Tab param aqui (sem indentar).
function tabLeavesAfterNormalEscape(getCM) {
  return Prec.highest(EditorView.domEventHandlers({
    keydown(event, view) {
      const state = getCM(view)?.state.vim
      if (!state || state.insertMode) return false
      if (event.key == "Escape" && !state.visualMode) view.setTabFocusMode(TAB_RELEASE_MS)
      return event.key == "Tab"
    },
  }))
}

// Rodapé do modo (--NORMAL--, --INSERT--, --VISUAL--) e linha de comando do : como a do terminal, nos tokens do tema;
// cursor de bloco no accent fora do insert.
const vimTheme = EditorView.theme({
  ".cm-panels": { backgroundColor: "var(--surface-sunken)", color: "var(--ink-muted)" },
  ".cm-panels-bottom": { borderTop: "1px solid var(--line)" },
  ".cm-vim-panel": { fontFamily: "inherit", fontSize: "0.75rem", lineHeight: "1.6", padding: "0 0.5rem" },
  ".cm-vim-panel input": { color: "var(--ink)", fontFamily: "inherit", fontSize: "inherit" },
  // O Vim pinta a letra sob o cursor com a cor do texto, inline; aqui ela inverte, como no terminal.
  "&.cm-editor .cm-fat-cursor": { background: "var(--accent)", color: "var(--surface) !important" },
  "&.cm-editor:not(.cm-focused) .cm-fat-cursor": { background: "none", outline: "1px solid var(--accent)", color: "transparent !important" },
})
