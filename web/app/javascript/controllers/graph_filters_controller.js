import { Controller } from "@hotwired/stimulus"
import { replaceQuery } from "controllers/url_query"

// Filtros do grafo cheio (graphs/_filters), como os do graph view do Obsidian. Cada troca é um passo do histórico com os
// filtros na query da página (pushState, sem visita: o foco fica onde estava, e recarregar ou compartilhar o endereço
// desenha igual) e pede ao canvas (outlet graph-view) o grafo filtrado. O voltar e o avançar entre esses passos marcam o
// painel pela query e redesenham, sem o Turbo carregar a página de novo: cada passo repete o estado do Turbo da entrada
// (a mesma visita para ele), e o voltar que chega a um passo desta visita para antes do Turbo (history:pop, de
// controllers/history_pop). Os de outra visita, ou com a tela já em outra página, seguem para o Turbo restaurar. A
// página não entra no cache do Turbo (graphs/show): a restauração sempre pede a página da query, nunca uma cópia com
// outros filtros. Teclas pelo code que a view tira do registro único
// (ShortcutsHelper): a do painel abre e fecha, a de cada filtro liga e desliga, a da comunidade vai para a próxima e,
// com Shift, para a anterior, e a de limpar mostra tudo (como o botão do vazio pelos filtros, no canvas). Valem com o
// foco na tela do grafo, ou no body, de onde a paleta repete a tecla, e não num campo de texto. Esc com o painel aberto o fecha antes de o Esc voltar à nota (area-back pula a camada data-esc-layer
// aberta); com o foco dentro do painel, ele volta ao summary, e fora dele (o canvas, depois do f) fica onde estava.
export default class extends Controller {
  static targets = [ "toggle", "community", "count" ]
  static outlets = [ "graph-view" ]
  static values = { code: String, clearCode: String }

  connect() {
    this.visitId = turboVisitId(window.history.state)
  }

  change() {
    const query = this.query()
    window.history.pushState(window.history.state, "", replaceQuery(new URL(window.location.href), query))
    this.show(query)
  }

  // Passo desta visita: o Turbo não o vê. ponytail: lê o restorationIdentifier do estado do History do Turbo 8.
  restore(event) {
    if (!this.visitId || turboVisitId(event.detail.state) != this.visitId) return

    event.preventDefault()
    this.mark(new URLSearchParams(window.location.search))
    this.show(this.query())
  }

  // Painel como a query diz: marcado o filtro que não vem false, e a comunidade dela (ou All).
  mark(params) {
    this.toggleTargets.forEach((toggle) => { toggle.checked = params.get(toggle.name) != "false" })
    this.communityTarget.value = params.get("community") ?? ""
    if (this.communityTarget.selectedIndex < 0) this.communityTarget.selectedIndex = 0
  }

  show(query) {
    const active = activeCount(query)
    this.countTarget.textContent = active ? `[${active}]` : ""
    if (!this.hasGraphViewOutlet) return

    this.graphViewOutlet.filteredValue = active > 0
    this.graphViewOutlet.redraw(query)
  }

  // Tudo marcado e todas as comunidades; sem filtro ligado, nada muda.
  clear() {
    if (!activeCount(this.query())) return

    this.mark(new URLSearchParams())
    this.change()
  }

  press(event) {
    if (!this.claims(event)) return

    const action = this.actionFor(event)
    if (!action) return

    event.preventDefault()
    action()
  }

  close(event) {
    if (!this.element.open || document.querySelector("dialog[open], :popover-open")) return

    event.preventDefault()
    const focusWasInside = this.element.contains(document.activeElement)
    this.element.open = false
    if (focusWasInside) this.element.querySelector("summary").focus()
  }

  query() {
    const hidden = this.toggleTargets.map((toggle) => [ toggle.name, toggle.checked ? null : "false" ])
    return { ...Object.fromEntries(hidden), community: this.communityTarget.value || null }
  }

  actionFor({ code, shiftKey }) {
    if (code == this.communityTarget.dataset.graphFiltersCode) return () => this.stepCommunity(shiftKey ? -1 : 1)
    if (shiftKey) return null
    if (code == this.codeValue) return () => { this.element.open = !this.element.open }
    if (code == this.clearCodeValue) return () => this.clear()

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

const turboVisitId = (state) => state?.turbo?.restorationIdentifier

const activeCount = (query) => Object.values(query).filter((value) => value != null).length
