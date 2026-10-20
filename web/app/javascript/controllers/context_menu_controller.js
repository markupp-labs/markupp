import { Controller } from "@hotwired/stimulus"

// Menu de contexto num popover (role=menu): abre num ponto da tela com o foco no primeiro item, setas ou j/k andam em
// volta, Home e End vão às pontas, Esc, Tab e clicar fora fecham. Fechado, o foco volta a quem abriu se ainda estava no
// menu; quem foi para outro lugar (o campo do caminho no Rename) fica onde está. O popover é manual: no auto, o
// pointerup do próprio botão direito (o contextmenu vem no pointerdown no Linux e no Mac) fechava o menu recém-aberto.
// Também abre por um botão com popovertarget (menu da conta no rodapé): o toggle leva o foco ao primeiro item, e o botão
// passa a ser quem abriu; clicar nele de novo fecha. Aberto pelo open(), quem abriu ganha data-menu-open até o menu fechar,
// para a linha da árvore mostrar sobre quem o menu age (tree.css).
const STEPS = { ArrowDown: 1, j: 1, ArrowUp: -1, k: -1 }
const EDGE_GAP = 4

export default class extends Controller {
  open({ x, y, opener }) {
    this.opener = opener
    opener.toggleAttribute("data-menu-open", true)
    if (this.element.matches(":popover-open")) this.element.hidePopover()
    this.element.showPopover()
    this.placeAt(x, y)
    this.items[0]?.focus()
  }

  close() {
    if (this.element.matches(":popover-open")) this.element.hidePopover()
  }

  closeOutside({ target }) {
    if (!this.element.contains(target) && !this.invoker?.contains(target)) this.close()
  }

  // Aberto pelo popovertarget, sem passar pelo open(): o foco ainda está fora do menu.
  focusFirst({ newState }) {
    if (newState != "open" || this.element.contains(document.activeElement)) return

    this.opener = this.invoker
    this.items[0]?.focus()
  }

  navigate(event) {
    if (event.key == "Tab" || event.key == "Escape") return this.closeFromKey(event)
    const step = STEPS[event.key]
    const edge = { Home: 0, End: this.items.length - 1 }[event.key]
    if ((step === undefined && edge === undefined) || event.ctrlKey || event.metaKey || event.altKey) return

    event.preventDefault()
    const index = edge ?? (this.items.indexOf(document.activeElement) + step + this.items.length) % this.items.length
    this.items[index].focus()
  }

  closeFromKey(event) {
    event.preventDefault()
    this.close()
  }

  returnFocus({ newState }) {
    if (newState != "closed") return
    this.opener?.removeAttribute("data-menu-open")
    if (!this.opener?.isConnected) return
    if (this.element.contains(document.activeElement) || document.activeElement == document.body) this.opener.focus()
  }

  // Abre para baixo e à direita do ponto, e encosta na borda quando não cabe (menu perto do fim da barra ou da tela).
  placeAt(x, y) {
    const { width, height } = this.element.getBoundingClientRect()
    this.element.style.left = `${Math.max(EDGE_GAP, Math.min(x, innerWidth - width - EDGE_GAP))}px`
    this.element.style.top = `${Math.max(EDGE_GAP, Math.min(y, innerHeight - height - EDGE_GAP))}px`
  }

  get invoker() {
    return document.querySelector(`[popovertarget="${this.element.id}"]`)
  }

  get items() {
    return [ ...this.element.querySelectorAll("[role=menuitem]:not([hidden])") ]
  }
}
