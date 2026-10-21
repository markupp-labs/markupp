import { Controller } from "@hotwired/stimulus"
import { EditorView } from "@codemirror/view"
import { inVimInsert } from "editor/vim_mode"

// Alt+h/j/k/l levam o foco à região vizinha, como o workbench.action.navigateLeft/Down/Up/Right do VS Code: a mais
// perto naquela direção pela geometria da tela, entre as visíveis marcadas com data-focus-region (árvore, caminho,
// editor, preview, lista de tenants, navegação das configurações). data-focus-region-target diz onde o foco cai, em
// seletores separados por ";" na ordem de preferência; sem ele, a própria região. Sem região naquela direção, nada.
// A tecla vem pelo code: no Mac o Option muda a letra (Option+h é o ˙). Campo de formulário, menu aberto e o insert do Vim ficam
// com a tecla, salvo as direções que o campo solta em data-focus-region-exits (o caminho solta a de baixo e a da direita).
// Um campo de texto recebe o foco com o cursor no fim, como o Shift+Tab do editor deixa o caminho. A origem é o foco, não o alvo do evento: a paleta repete o atalho no body depois de devolver o foco.
const DIRECTIONS = { KeyH: "left", KeyJ: "down", KeyK: "up", KeyL: "right" }

export default class extends Controller {
  navigate(event) {
    const direction = directionOf(event)
    if (!direction || keepsKeys(event.target, direction)) return

    event.preventDefault()
    event.stopPropagation()
    const origin = document.activeElement?.closest("[data-focus-region]") ?? document.activeElement
    if (origin && origin != document.body) enter(nearestRegion(origin, direction))
  }
}

function directionOf(event) {
  if (!event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || event.defaultPrevented) return null
  return DIRECTIONS[event.code]
}

function keepsKeys(target, direction) {
  if (target.dataset?.focusRegionExits?.split(" ").includes(direction)) return false
  return target.closest?.("input, textarea, select, dialog, [role=menu]") != null || inVimInsert(target)
}

// O editor volta pelo view.focus(), que repõe o cursor onde estava; o resto recebe o foco direto.
function enter(region) {
  const target = region && focusTarget(region)
  if (!target) return

  if (target.matches(".cm-content")) return EditorView.findFromDOM(target)?.focus()

  target.focus()
  if (target.matches("input[type=text]")) target.setSelectionRange(target.value.length, target.value.length)
}

function focusTarget(region) {
  const selectors = region.dataset.focusRegionTarget?.split(";")
  if (!selectors) return region
  return selectors.flatMap((selector) => [ ...region.querySelectorAll(selector) ]).find((element) => element.checkVisibility())
}

function nearestRegion(origin, direction) {
  const from = origin.getBoundingClientRect()
  return [ ...document.querySelectorAll("[data-focus-region]") ]
    .filter((region) => region != origin && region.checkVisibility() && focusTarget(region))
    .map((region) => ({ region, rank: rankOf(from, region.getBoundingClientRect(), direction) }))
    .filter(({ rank }) => rank)
    .sort((a, b) => a.rank[0] - b.rank[0] || a.rank[1] - b.rank[1])[0]?.region
}

// Só conta quem está naquela direção (a borda de cá depois da borda de lá, com 2px de folga). Primeiro quem cruza a
// faixa da origem (a altura dela para h e l, a largura para j e k); depois a distância até a borda mais a distância
// entre os centros na outra direção, para a árvore ir ao editor e não ao caminho, que fica no alto.
function rankOf(from, to, direction) {
  const horizontal = direction == "left" || direction == "right"
  const gap = { left: from.left - to.right, right: to.left - from.right, up: from.top - to.bottom, down: to.top - from.bottom }[direction]
  if (gap < -2) return null

  const [ start, end ] = horizontal ? [ "top", "bottom" ] : [ "left", "right" ]
  const overlap = Math.min(from[end], to[end]) - Math.max(from[start], to[start])
  const centerDistance = Math.abs((from[start] + from[end]) / 2 - (to[start] + to[end]) / 2)
  return [ overlap > 0 ? 0 : 1, Math.max(gap, 0) + centerDistance ]
}
