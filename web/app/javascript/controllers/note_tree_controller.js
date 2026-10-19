import { Controller } from "@hotwired/stimulus"

// Árvore de notas da barra lateral (notes/_folder.html.erb), como o treeview de navegação da WAI: um Tab só para a
// árvore (roving tabindex, na nota atual ou na primeira linha), setas ou j/k sobem e descem pelas linhas visíveis,
// l ou seta direita abre a pasta e entra nela, h ou seta esquerda fecha e sobe ao pai. Enter abre a nota (o link) ou
// alterna a pasta (o botão).
const MOVES = {
  ArrowDown: "next", j: "next", ArrowUp: "previous", k: "previous",
  ArrowRight: "expand", l: "expand", ArrowLeft: "collapse", h: "collapse", Home: "first", End: "last"
}

export default class extends Controller {
  connect() {
    this.rove(this.element.querySelector("[aria-current=page]") ?? this.rows[0])
  }

  // Clicar fecha ou abre e deixa a parada do Tab na pasta: a linha que tinha a parada pode ter sumido no grupo fechado.
  toggle({ currentTarget }) {
    setExpanded(currentTarget, !isExpanded(currentTarget))
    this.rove(currentTarget)
  }

  // A linha que ganha o foco (teclado ou clique) passa a ser a parada do Tab.
  roveToFocused({ target }) {
    if (target.matches("[role=treeitem]")) this.rove(target)
  }

  navigate(event) {
    const move = MOVES[event.key]
    if (!move || event.ctrlKey || event.metaKey || event.altKey) return

    event.preventDefault()
    this[move](event.target.closest("[role=treeitem]"))
  }

  next(row) {
    this.focus(this.rows[this.rows.indexOf(row) + 1])
  }

  previous(row) {
    this.focus(this.rows[this.rows.indexOf(row) - 1])
  }

  first() {
    this.focus(this.rows[0])
  }

  last() {
    this.focus(this.rows.at(-1))
  }

  // Pasta fechada abre; aberta, o foco desce ao primeiro filho. Nota não tem para onde ir.
  expand(row) {
    if (!row.hasAttribute("aria-expanded")) return
    if (!isExpanded(row)) return setExpanded(row, true)

    this.focus(groupOf(row).querySelector("[role=treeitem]"))
  }

  // Pasta aberta fecha; senão o foco sobe à pasta que contém a linha.
  collapse(row) {
    if (isExpanded(row)) return setExpanded(row, false)

    const group = row.closest("[role=group]")
    if (group) this.focus(this.element.querySelector(`[aria-owns="${group.id}"]`))
  }

  focus(row) {
    row?.focus()
  }

  rove(row) {
    if (!row) return

    this.element.querySelectorAll("[role=treeitem]").forEach((item) => (item.tabIndex = item == row ? 0 : -1))
  }

  // As linhas na ordem da tela, sem as que estão dentro de pasta fechada.
  get rows() {
    return [ ...this.element.querySelectorAll("[role=treeitem]") ].filter((row) => !row.closest("[role=group][hidden]"))
  }
}

function isExpanded(row) {
  return row.getAttribute("aria-expanded") == "true"
}

function groupOf(folder) {
  return document.getElementById(folder.getAttribute("aria-owns"))
}

function setExpanded(folder, expanded) {
  folder.setAttribute("aria-expanded", String(expanded))
  groupOf(folder).hidden = !expanded
}
