// Atalho de uma tecla só (?, /, g t) não vale com o foco num campo de texto: ali a tecla é texto.
export function isTyping(target) {
  return target.closest("input, textarea, select, [contenteditable]") != null
}

// Com o modo Vim ligado, o foco no editor deixa para o Vim (editor/vim_mode.js) as teclas que ele usa (Ctrl+B, Ctrl+E,
// Ctrl+\), como o VSCodeVim: esses atalhos do painel valem fora do editor e pela paleta. Os que o Vim não usa (Ctrl+S,
// a paleta, a busca, Ctrl+, , Alt+N) valem também no editor. Sem o Vim, o editor não tem a marca e nada muda.
export function inVimEditor(target) {
  return target.closest?.("[data-vim-editor]") != null
}
