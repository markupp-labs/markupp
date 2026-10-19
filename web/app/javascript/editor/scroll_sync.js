// Rolagem do lado a lado, como no VS Code: o preview mostra o mesmo trecho do documento que o editor.
// Cada bloco do preview traz data-source-line (render_markdown.js); o topo do preview fica entre os dois blocos
// que cercam a primeira linha visível do editor, na mesma proporção.
// ponytail: mede todos os blocos a cada quadro; cachear as posições se nota muito longa pesar.
export function previewScrollTopFor(view, preview) {
  const scroller = view.scrollDOM
  if (scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 1) return preview.scrollHeight

  const line = editorTopLine(view)
  const anchors = sourceAnchors(preview, view.state.doc.lines)
  const next = anchors.findIndex((anchor) => anchor.line > line)
  return interpolate(anchors[next - 1], anchors[next], line)
}

// Linha (base 0, como o map do markdown-it) no topo do editor, com a fração já rolada dela.
function editorTopLine(view) {
  const top = Math.max(view.scrollDOM.scrollTop - view.documentPadding.top, 0)
  const block = view.lineBlockAtHeight(top)
  const fraction = block.height ? (top - block.top) / block.height : 0
  return view.state.doc.lineAt(block.from).number - 1 + Math.min(Math.max(fraction, 0), 1)
}

// Posições dos blocos no preview, entre um começo (linha 0 no topo) e um fim (última linha no fundo).
function sourceAnchors(preview, lineCount) {
  const origin = preview.getBoundingClientRect().top - preview.scrollTop
  const blocks = [ ...preview.querySelectorAll("[data-source-line]") ].map((block) => ({
    line: Number(block.dataset.sourceLine), top: block.getBoundingClientRect().top - origin,
  }))
  return [ { line: 0, top: 0 }, ...blocks, { line: lineCount, top: preview.scrollHeight } ]
}

function interpolate(before, after, line) {
  if (!after || after.line == before.line) return before.top

  return before.top + (after.top - before.top) * (line - before.line) / (after.line - before.line)
}
