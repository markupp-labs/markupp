import { Controller } from "@hotwired/stimulus"
import { isTyping } from "controllers/typing"

// Lista de tenants em cards (tenants/_grid): / vai à busca, Alt+N abre o tenant novo, setas ou h/j/k/l andam entre os
// cards pela grade (sem card embaixo, j vai ao seguinte; sem card em cima, k ao anterior, e do primeiro volta à busca),
// Home e End vão às pontas. Sem foco em card, a primeira seta ou j/k foca o tenant ativo. Na busca, ↓ desce aos cards
// e Enter entra no primeiro.
const MOVES = {
  ArrowDown: "down", j: "down", ArrowUp: "up", k: "up", ArrowRight: "right", l: "right", ArrowLeft: "left", h: "left",
  Home: "first", End: "last"
}

export default class extends Controller {
  static targets = [ "filter", "card", "empty", "newTenant" ]
  static values = { openNew: Boolean }

  connect() {
    if (this.openNewValue) this.newTenantTarget.showModal()
  }

  // A tecla do Alt+N vem pelo code: no Mac o Option muda a letra.
  shortcut(event) {
    if (event.defaultPrevented || event.ctrlKey || event.metaKey) return
    if (event.altKey) return event.code == "KeyN" && !event.shiftKey && this.openNew(event)
    if (isTyping(event.target) || this.newTenantTarget.open) return
    if (event.key == "/") return this.focusOn(event, this.filterTarget)
    if (MOVES[event.key] && document.activeElement == document.body) this.focusOn(event, this.opener(this.activeCard))
  }

  openNew(event) {
    event.preventDefault()
    this.newTenantTarget.showModal()
  }

  navigate(event) {
    const move = MOVES[event.key]
    if (!move || !event.target.matches(".tenant-card-open") || event.ctrlKey || event.metaKey || event.altKey) return

    const cards = this.visibleCards
    const target = stepIndex(cards, cards.indexOf(event.target.closest("li")), move)
    if (target < 0) return this.focusOn(event, move == "up" ? this.filterTarget : null)
    this.focusOn(event, this.opener(cards[Math.min(target, cards.length - 1)]))
  }

  filter() {
    const query = this.filterTarget.value.trim().toLowerCase()
    this.cardTargets.forEach((card) => (card.hidden = !card.dataset.tenantName.includes(query)))
    this.emptyTarget.hidden = this.visibleCards.length > 0
  }

  leaveFilter(event) {
    const first = this.opener(this.visibleCards[0])
    if (event.key == "ArrowDown" && first) this.focusOn(event, first)
    if (event.key == "Enter" && first) first.click()
  }

  focusOn(event, element) {
    event.preventDefault()
    element?.focus()
  }

  opener(card) {
    return card?.querySelector(".tenant-card-open")
  }

  get activeCard() {
    return this.visibleCards.find((card) => card.querySelector("[aria-current=true]")) ?? this.visibleCards[0]
  }

  get visibleCards() {
    return this.cardTargets.filter((card) => !card.hidden)
  }
}

// Índice do card de destino pela grade: as colunas são os cards na altura do primeiro. Antes do primeiro dá -1.
function stepIndex(cards, index, move) {
  const columns = cards.filter((card) => card.offsetTop == cards[0].offsetTop).length
  const below = index + columns < cards.length ? index + columns : index + 1
  const above = index - columns >= 0 ? index - columns : index - 1
  return { down: below, up: above, right: index + 1, left: index - 1, first: 0, last: cards.length - 1 }[move]
}
