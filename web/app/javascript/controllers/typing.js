// Atalho de uma tecla só (?, /, g t) não vale com o foco num campo de texto: ali a tecla é texto.
export function isTyping(target) {
  return target.closest("input, textarea, select, [contenteditable]") != null
}

// Com o modo Vim ligado, o foco no editor deixa as teclas para o Vim (editor/vim_mode.js), como o VSCodeVim: os atalhos
// do painel valem fora do editor e pela paleta. Sem o Vim, o editor não tem a marca e nada muda.
export function inVimEditor(target) {
  return target.closest?.("[data-vim-editor]") != null
}
