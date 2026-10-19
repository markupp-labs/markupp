// Atalho de uma tecla só (?, /, g t) não vale com o foco num campo de texto: ali a tecla é texto.
export function isTyping(target) {
  return target.closest("input, textarea, select, [contenteditable]") != null
}
