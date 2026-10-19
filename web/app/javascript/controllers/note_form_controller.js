import { Controller } from "@hotwired/stimulus"

// Salvar explícito: Ctrl+S ou Cmd+S submete, marca alterações não salvas e avisa antes de sair com elas.
// Com aviso de conflito ou erro de caminho na tela, o que o usuário digitou ainda não está no servidor.
// O caminho conta como alteração: renomear e mover também precisam salvar.
// Nota nova sem nada escrito não é alteração: sair dela a descarta sem perguntar.
// Não há nome automático: com o nome vazio (campo em branco ou barra no fim) o formulário não sai do navegador.
// A resolução do caminho (relativo, absoluto, .md) é só do servidor (Note#resolve); aqui só o nome vazio.
export default class extends Controller {
  static targets = [ "source", "path", "unsaved", "save", "pending", "pathError" ]
  static values = { leaveMessage: String, new: Boolean, blankTitle: String }

  connect() {
    this.originals = this.fields.map((field) => field.defaultValue)
    const heading = headingFor(this.pathTarget.value)
    this.heading = this.newValue && this.sourceTarget.value.startsWith(heading) ? heading : ""
    if (this.newValue) this.moveCaretToEnd()
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

  // Só na nota nova: o "# nome" no começo do conteúdo acompanha o caminho, enquanto o usuário não mexer nele.
  // O editor troca o cabeçalho anterior pelo novo e marca applied; se o começo mudou, deixa como está.
  syncHeading() {
    if (!this.newValue) return

    const next = headingFor(this.pathTarget.value)
    const { detail } = this.dispatch("heading", { detail: { from: this.heading, to: next, applied: false } })
    if (detail.applied) this.heading = next
  }

  // Shift+Tab no conteúdo volta ao caminho, com o cursor no fim dele.
  focusPath(event) {
    if (!event.target.closest(".cm-editor")) return

    event.preventDefault()
    this.pathTarget.focus()
    this.moveCaretToEnd()
  }

  // A nota nova abre com a pasta no campo (/work/): o que se digita entra depois dela.
  moveCaretToEnd() {
    this.pathTarget.setSelectionRange(this.pathTarget.value.length, this.pathTarget.value.length)
  }

  // Nome em branco: avisa sob o campo e devolve o foco a ele, com o caminho digitado e o conteúdo como estão.
  submit(event) {
    if (nameOf(this.pathTarget.value)) return this.allowLeaving()

    event.preventDefault()
    this.refuseBlankName()
  }

  refuseBlankName() {
    this.pathErrorTarget.textContent = this.blankTitleValue
    this.pathErrorTarget.hidden = false
    this.pathTarget.setAttribute("aria-invalid", "true")
    this.pathTarget.setAttribute("aria-describedby", this.pathErrorTarget.id)
    this.pathTarget.focus()
  }

  // O aviso de nome em branco sai assim que há nome; outro erro de caminho fica até o servidor responder.
  clearBlankName() {
    if (!nameOf(this.pathTarget.value) || this.pathErrorTarget.textContent != this.blankTitleValue) return

    this.pathErrorTarget.textContent = ""
    this.pathErrorTarget.hidden = true
    this.pathTarget.removeAttribute("aria-invalid")
    this.pathTarget.removeAttribute("aria-describedby")
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
    return [ this.sourceTarget, this.pathTarget ]
  }

  get changed() {
    const edited = this.fields.some((field, index) => field.value != this.originals[index])
    return !this.leaving && (this.hasPendingTarget || edited)
  }
}

// O nome da nota no caminho digitado, como o servidor lê: o último segmento, sem o .md final; vazio com barra no fim.
function nameOf(path) {
  return path.trim().replace(/\.md$/, "").split("/").pop()
}

// Só o editor põe o cabeçalho; o servidor grava o conteúdo como veio. Leva só o nome: pastas e .md não entram.
function headingFor(path) {
  const name = nameOf(path).trim()
  return name ? `# ${name}\n\n` : ""
}
