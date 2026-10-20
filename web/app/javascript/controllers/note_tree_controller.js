import { Controller } from "@hotwired/stimulus"
import { rememberSubmitOrigin, submitFrom, visitFrom } from "motion/busy_origin"

// Árvore de notas da barra lateral (notes/_folder.html.erb), como o treeview de navegação da WAI: um Tab só para a
// árvore (roving tabindex, na nota atual ou na primeira linha), setas ou j/k sobem e descem pelas linhas visíveis,
// l ou seta direita abre a pasta e entra nela, h ou seta esquerda fecha e sobe ao pai. Enter abre a nota (o link) ou
// alterna a pasta (o botão).
// Ações da linha, pelo menu de contexto (notes/_tree_menu.html.erb) ou pela tecla com o foco nela: nova nota na pasta,
// renomear (F2), apagar (Delete), copiar o caminho (Alt+Shift+C) e o wikilink (Alt+Shift+W). Alt+N de qualquer lugar
// abre a nota nova na pasta da nota aberta. A tecla vem pelo code: no Mac o Option muda a letra (Option+N é o til).
const MOVES = {
  ArrowDown: "next", j: "next", ArrowUp: "previous", k: "previous",
  ArrowRight: "expand", l: "expand", ArrowLeft: "collapse", h: "collapse", Home: "first", End: "last"
}

// Espelho das teclas do ShortcutsHelper (grupo tree); o shortcuts_helper_test confere que batem.
const ROW_SHORTCUTS = { F2: "rename", Delete: "delete", "alt+shift+KeyC": "copyPath", "alt+shift+KeyW": "copyWikilink" }
const STATUS_TIMEOUT = 2000

export default class extends Controller {
  static targets = [ "status", "deleteForm" ]
  static outlets = [ "context-menu" ]
  static values = { newNoteUrl: String, copied: String, copyFailed: String }

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
    if (!isFolder(row)) return
    if (!isExpanded(row)) return setExpanded(row, true)

    this.focus(groupOf(row).querySelector("[role=treeitem]"))
  }

  // Pasta aberta fecha; senão o foco sobe à pasta que contém a linha.
  collapse(row) {
    if (isExpanded(row)) return setExpanded(row, false)

    const group = row.closest("[role=group]")
    if (group) this.focus(this.element.querySelector(`[aria-owns="${group.id}"]`))
  }

  // Só quem escreve tem o url da nota nova; com o foco numa linha, a nota nova vai para a pasta dela.
  newNote(event) {
    if (event.code != "KeyN" || !event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || event.defaultPrevented) return
    if (!this.hasNewNoteUrlValue) return

    event.preventDefault()
    this.visitNewNote(this.rowOf(event.target))
  }

  rowShortcut(event) {
    const row = event.target.closest("[role=treeitem]")
    if (event.key == "ContextMenu" || (event.key == "F10" && event.shiftKey)) return this.openMenuAtRow(event, row)

    const action = ROW_SHORTCUTS[event.altKey && event.shiftKey ? `alt+shift+${event.code}` : event.key]
    if (!action || !this.allows(action, row)) return

    event.preventDefault()
    if (action == "delete") return this.deleteRow(row)
    this.menuRow = row
    this[action]()
  }

  openMenuAtPointer(event) {
    const row = event.target.closest("[role=treeitem]")
    if (!row) return

    event.preventDefault()
    this.openMenu(row, event.clientX, event.clientY)
  }

  openMenuAtRow(event, row) {
    event.preventDefault()
    const { left, bottom } = row.getBoundingClientRect()
    this.openMenu(row, left + 16, bottom)
  }

  openMenu(row, x, y) {
    this.menuRow = row
    this.contextMenuOutlet.element.querySelectorAll("[data-note-only=true]").forEach((item) => (item.hidden = isFolder(row)))
    this.contextMenuOutlet.open({ x, y, opener: row })
  }

  // Ações do menu, sobre a linha em que ele abriu.
  newHere() {
    this.visitNewNote(this.menuRow)
  }

  // Na nota aberta só foca e seleciona o campo, sem recarregar o que não foi salvo; outra nota abre já assim (?rename).
  rename() {
    const field = document.getElementById("note_path")
    if (this.menuRow.matches("[aria-current=page]") && field) return selectAll(field)

    const url = new URL(this.menuRow.href)
    url.searchParams.set("rename", "")
    visitFrom(this.menuRow, url)
  }

  // O item do menu é o submit do form; aqui só aponta o form para a nota. A confirmação é do Turbo (data-turbo-confirm).
  delete() {
    this.deleteFormTarget.action = this.menuRow.href
    rememberSubmitOrigin(this.deleteFormTarget, this.menuRow)
  }

  deleteRow(row) {
    this.deleteFormTarget.action = row.href
    submitFrom(row, this.deleteFormTarget)
  }

  copyPath() {
    this.copy(this.menuRow.dataset.path)
  }

  copyWikilink() {
    this.copy(`[[${this.menuRow.dataset.path.split("/").pop()}]]`)
  }

  async copy(text) {
    try {
      await navigator.clipboard.writeText(text)
      // Função no replace: o caminho entra como texto, sem $& ou $$ virarem padrão de substituição.
      this.statusTarget.textContent = this.copiedValue.replace("%{text}", () => text)
    } catch {
      this.statusTarget.textContent = this.copyFailedValue
    }
    clearTimeout(this.statusTimer)
    this.statusTimer = setTimeout(() => (this.statusTarget.textContent = ""), STATUS_TIMEOUT)
  }

  // A pasta da linha (a própria pasta, ou a da nota); sem linha, o url do servidor já leva a pasta da nota aberta.
  visitNewNote(row) {
    const url = new URL(this.newNoteUrlValue, location.href)
    if (row) {
      const folder = (isFolder(row) ? row.dataset.path : row.dataset.path.replace(/\/[^/]*$/, "")).slice(1)
      folder ? url.searchParams.set("folder", folder) : url.searchParams.delete("folder")
    }
    visitFrom(row, url)
  }

  // Quem só lê copia; renomear e apagar são de quem escreve, e só valem para nota, como o wikilink.
  allows(action, row) {
    if (!row) return false
    if ([ "rename", "delete" ].includes(action) && !this.hasNewNoteUrlValue) return false
    return action == "copyPath" || !isFolder(row)
  }

  rowOf(element) {
    return this.element.contains(element) ? element.closest("[role=treeitem]") : null
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

function isFolder(row) {
  return row.hasAttribute("aria-expanded")
}

function selectAll(field) {
  field.focus()
  field.select()
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
