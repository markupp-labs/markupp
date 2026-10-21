import { Controller } from "@hotwired/stimulus"

// Visão Links da nota aberta (note_links/show), no frame que o Turbo mantém: setas e j/k andam pelas seções e linhas
// visíveis (as de uma seção dobrada ficam de fora), Enter abre a nota ou dobra a seção (o próprio link e o summary), y
// numa sugestão copia o wikilink dela, como o yank do Vim. Teclas pelo code. Quem apertou Ctrl+Shift+L antes de o frame
// chegar ficou com o foco na visão; quando ele chega, o foco desce para a primeira seção. O Show more no fim de uma seção
// cheia carrega a página seguinte no frame dele; o link sai e o foco desce para a primeira linha nova.
const STEPS = { ArrowDown: 1, KeyJ: 1, ArrowUp: -1, KeyK: -1 }
const ITEMS = "summary, [data-links-item]"
const STATUS_TIMEOUT = 2000

export default class extends Controller {
  static targets = [ "status" ]
  static values = { copied: String, copyFailed: String }

  navigate(event) {
    if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return
    if (event.code == "KeyY") return this.yank(event)

    const step = STEPS[event.code]
    if (!step) return

    event.preventDefault()
    const items = this.items
    const index = items.indexOf(event.target.closest("li")?.querySelector(ITEMS) ?? event.target)
    items[Math.min(Math.max(index + step, 0), items.length - 1)]?.focus()
  }

  frameLoaded({ target }) {
    if (target == this.element) return this.focusWhenWaiting()
    if (document.activeElement != document.body) return

    const first = target.querySelector(ITEMS) ?? target.closest("details")?.querySelector("summary")
    first?.focus()
  }

  focusWhenWaiting() {
    if (document.activeElement == this.element.closest("[role=tabpanel]")) this.items[0]?.focus()
  }

  copy({ currentTarget }) {
    this.writeClipboard(currentTarget.dataset.wikilink)
  }

  yank(event) {
    const wikilink = event.target.dataset.wikilink
    if (!wikilink) return

    event.preventDefault()
    this.writeClipboard(wikilink)
  }

  async writeClipboard(text) {
    try {
      await navigator.clipboard.writeText(text)
      // Função no replace: o wikilink entra como texto, sem $& ou $$ virarem padrão de substituição.
      this.statusTarget.textContent = this.copiedValue.replace("%{text}", () => text)
    } catch {
      this.statusTarget.textContent = this.copyFailedValue
    }
    clearTimeout(this.statusTimer)
    this.statusTimer = setTimeout(() => (this.statusTarget.textContent = ""), STATUS_TIMEOUT)
  }

  get items() {
    return [ ...this.element.querySelectorAll(ITEMS) ].filter((item) => item.checkVisibility())
  }
}
