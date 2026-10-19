import { Controller } from "@hotwired/stimulus"

// Salvar explícito: Ctrl+S ou Cmd+S submete, marca alterações não salvas e avisa antes de sair com elas.
// Com aviso de conflito ou erro de caminho na tela, o que o usuário digitou ainda não está no servidor.
// Pasta e título contam como alteração: renomear e mover também precisam salvar.
// Nota nova sem nada escrito não é alteração: sair dela a descarta sem perguntar.
// O título é o nome do arquivo e não há nome automático: sem ele o formulário não sai do navegador.
export default class extends Controller {
  static targets = [ "source", "title", "folder", "unsaved", "save", "pending", "titleError" ]
  static values = { leaveMessage: String, new: Boolean, blankTitle: String }

  connect() {
    this.originals = this.fields.map((field) => field.defaultValue)
    const heading = headingFor(this.titleTarget.value)
    this.heading = this.newValue && this.sourceTarget.value.startsWith(heading) ? heading : ""
    this.track()
  }

  // Nota só de leitura (papel viewer) ou sem mudança não tem o que salvar.
  save() {
    if (this.sourceTarget.readOnly || !this.changed) return

    this.element.requestSubmit()
  }

  track() {
    this.unsavedTarget.hidden = !this.changed
    if (this.hasSaveTarget) this.saveTarget.hidden = !this.changed
  }

  // Só na nota nova: o "# título" no começo do conteúdo acompanha o título, enquanto o usuário não mexer nele.
  // O editor troca o cabeçalho anterior pelo novo e marca applied; se o começo mudou, deixa como está.
  syncHeading() {
    if (!this.newValue) return

    const next = headingFor(this.titleTarget.value)
    const { detail } = this.dispatch("heading", { detail: { from: this.heading, to: next, applied: false } })
    if (detail.applied) this.heading = next
  }

  // Shift+Tab no conteúdo volta ao título, com o cursor no fim dele.
  focusTitle(event) {
    if (!event.target.closest(".cm-editor")) return

    event.preventDefault()
    this.titleTarget.focus()
    this.titleTarget.setSelectionRange(this.titleTarget.value.length, this.titleTarget.value.length)
  }

  // Título em branco: avisa sob o campo e devolve o foco a ele, com pasta e conteúdo como estão.
  submit(event) {
    if (this.titleTarget.value.trim()) return this.allowLeaving()

    event.preventDefault()
    this.refuseBlankTitle()
  }

  refuseBlankTitle() {
    this.titleErrorTarget.textContent = this.blankTitleValue
    this.titleErrorTarget.hidden = false
    this.titleTarget.setAttribute("aria-invalid", "true")
    this.titleTarget.setAttribute("aria-describedby", this.titleErrorTarget.id)
    this.titleTarget.focus()
  }

  // O aviso de título em branco sai assim que há título; outro erro de caminho fica até o servidor responder.
  clearBlankTitle() {
    if (!this.titleTarget.value.trim() || this.titleErrorTarget.textContent != this.blankTitleValue) return

    this.titleErrorTarget.textContent = ""
    this.titleErrorTarget.hidden = true
    this.titleTarget.removeAttribute("aria-invalid")
    this.titleTarget.removeAttribute("aria-describedby")
  }

  allowLeaving() {
    this.leaving = true
  }

  // O form de excluir fica fora deste (form não aninha); só envia depois do confirm aceito.
  allowLeavingOnDelete({ target }) {
    if (target.id == "delete_note") this.allowLeaving()
  }

  warnBeforeUnload(event) {
    if (this.changed) event.preventDefault()
  }

  confirmVisit(event) {
    if (this.changed && !confirm(this.leaveMessageValue)) event.preventDefault()
  }

  get fields() {
    return [ this.sourceTarget, this.titleTarget, this.folderTarget ]
  }

  get changed() {
    const edited = this.fields.some((field, index) => field.value != this.originals[index])
    return !this.leaving && (this.hasPendingTarget || edited)
  }
}

// Só o editor põe o cabeçalho; o servidor grava o conteúdo como veio. O título é o nome do arquivo: o que vem antes
// da última barra é pasta e não entra; com barra no fim não há nome, e nem cabeçalho.
function headingFor(title) {
  const name = title.trim().split("/").pop().trim()
  return name ? `# ${name}\n\n` : ""
}
