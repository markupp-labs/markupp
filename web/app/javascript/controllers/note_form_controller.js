import { Controller } from "@hotwired/stimulus"

// Salvar explícito: Ctrl+S ou Cmd+S submete, marca alterações não salvas e avisa antes de sair com elas.
// Com aviso de conflito ou erro de caminho na tela, o que o usuário digitou ainda não está no servidor.
// O caminho conta como alteração: renomear também precisa salvar.
export default class extends Controller {
  static targets = [ "source", "path", "unsaved", "pending" ]
  static values = { leaveMessage: String }

  connect() {
    this.originalContent = this.sourceTarget.defaultValue
    this.originalPath = this.pathTarget.defaultValue
    this.track()
  }

  save() {
    this.element.requestSubmit()
  }

  track() {
    this.unsavedTarget.hidden = !this.changed
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

  get changed() {
    const edited = this.sourceTarget.value != this.originalContent || this.pathTarget.value != this.originalPath
    return !this.leaving && (this.hasPendingTarget || edited)
  }
}
