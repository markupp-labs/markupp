// Rolagem do lado a lado, como no VS Code: um painel mostra o mesmo trecho do documento que o outro.
// Cada bloco do preview traz data-source-line (render_markdown.js); entre os dois blocos que cercam o topo de um
// painel, o outro fica na mesma proporção (interpolação). Começo e fim de cada lado servem de âncoras extras.
// ponytail: mede todos os blocos a cada quadro; cachear as posições se nota muito longa pesar.
export function previewScrollTopFor(view, preview) {
  if (atBottom(view.scrollDOM)) return preview.scrollHeight

  const line = editorTopLine(view)
  const anchors = sourceAnchors(preview, view.state.doc.lines)
  const next = anchors.findIndex((anchor) => anchor.line > line)
  return interpolate(anchors[next - 1], anchors[next], "line", line, "top")
}

// O caminho inverso: a linha do markdown no topo do preview vira a altura dela no editor.
export function editorScrollTopFor(view, preview) {
  if (atBottom(preview)) return view.scrollDOM.scrollHeight

  const anchors = sourceAnchors(preview, view.state.doc.lines)
  const next = anchors.findIndex((anchor) => anchor.top > preview.scrollTop)
  return editorTopOf(view, interpolate(anchors[next - 1], anchors[next], "top", preview.scrollTop, "line"))
}

const atBottom = (scroller) => scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 1

// Linha (base 0, como o map do markdown-it) no topo do editor, com a fração já rolada dela.
function editorTopLine(view) {
  const top = Math.max(view.scrollDOM.scrollTop - view.documentPadding.top, 0)
  const block = view.lineBlockAtHeight(top)
  const fraction = block.height ? (top - block.top) / block.height : 0
  return view.state.doc.lineAt(block.from).number - 1 + Math.min(Math.max(fraction, 0), 1)
}

function editorTopOf(view, line) {
  const whole = Math.min(Math.floor(line), view.state.doc.lines - 1)
  const block = view.lineBlockAt(view.state.doc.line(whole + 1).from)
  return block.top + block.height * (line - whole) + view.documentPadding.top
}

// Posições dos blocos no preview, entre um começo (linha 0 no topo) e um fim (última linha no fundo).
function sourceAnchors(preview, lineCount) {
  const origin = preview.getBoundingClientRect().top - preview.scrollTop
  const blocks = [ ...preview.querySelectorAll("[data-source-line]") ].map((block) => ({
    line: Number(block.dataset.sourceLine), top: block.getBoundingClientRect().top - origin,
  }))
  return [ { line: 0, top: 0 }, ...blocks, { line: lineCount, top: preview.scrollHeight } ]
}

// Valor de want entre as âncoras before e after, na posição de value na escala de by.
function interpolate(before, after, by, value, want) {
  if (!after || after[by] == before[by]) return before[want]

  return before[want] + (after[want] - before[want]) * (value - before[by]) / (after[by] - before[by])
}
