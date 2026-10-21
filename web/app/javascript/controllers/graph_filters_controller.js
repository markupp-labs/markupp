import { Controller } from "@hotwired/stimulus"
import { replaceQuery } from "controllers/url_query"

// Filtros do grafo cheio (graphs/_filters), como os do graph view do Obsidian. Cada troca põe os filtros na query da
// página (replaceState, sem visita: o foco fica onde estava, e recarregar ou compartilhar o endereço desenha igual) e
// pede ao canvas (outlet graph-view) o grafo filtrado. Teclas pelo code que a view tira do registro único
// (ShortcutsHelper): a do painel abre e fecha, a de cada filtro liga e desliga, a da comunidade vai para a próxima e,
// com Shift, para a anterior. Valem com o foco na tela do grafo, ou no body, de onde a paleta repete a tecla, e não num
// campo de texto. Esc dentro do painel o fecha e devolve o foco ao summary, antes de o Esc voltar à nota.
export default class extends Controller {
  static targets = [ "toggle", "community", "count" ]
  static outlets = [ "graph-view" ]
  static values = { code: String }

  change() {
    const query = this.query()
    window.history.replaceState(window.history.state, "", replaceQuery(new URL(window.location.href), query))
    const active = Object.values(query).filter((value) => value != null).length
    this.countTarget.textContent = active ? `[${active}]` : ""
    if (this.hasGraphViewOutlet) this.graphViewOutlet.redraw(query)
  }

  press(event) {
    if (!this.claims(event)) return

    const action = this.actionFor(event)
    if (!action) return

    event.preventDefault()
    action()
  }

  close(event) {
    if (!this.element.open) return

    event.preventDefault()
    this.element.open = false
    this.element.querySelector("summary").focus()
  }

  query() {
    const hidden = this.toggleTargets.map((toggle) => [ toggle.name, toggle.checked ? null : "false" ])
    return { ...Object.fromEntries(hidden), community: this.communityTarget.value || null }
  }

  actionFor({ code, shiftKey }) {
    if (code == this.communityTarget.dataset.graphFiltersCode) return () => this.stepCommunity(shiftKey ? -1 : 1)
    if (shiftKey) return null
    if (code == this.codeValue) return () => { this.element.open = !this.element.open }

    const toggle = this.toggleTargets.find((target) => target.dataset.graphFiltersCode == code)
    return toggle && (() => { toggle.checked = !toggle.checked; this.change() })
  }

  stepCommunity(direction) {
    const select = this.communityTarget
    select.selectedIndex = (select.selectedIndex + direction + select.length) % select.length
    this.change()
  }

  // A tecla é do grafo: sem Ctrl, Alt ou Meta, ninguém a tratou antes e o foco está na tela do grafo fora de campo de
  // texto (a caixa de um filtro conta como tela do grafo).
  claims({ target, defaultPrevented, ctrlKey, altKey, metaKey }) {
    if (defaultPrevented || ctrlKey || altKey || metaKey) return false
    if (target.closest?.("textarea, select, [contenteditable], input:not([type=checkbox])")) return false
    return target == document.body || this.element.closest(".graph-page").contains(target)
  }
}
