import { Controller } from "@hotwired/stimulus"

// Salvar explícito: Ctrl+S ou Cmd+S submete, marca alterações não salvas e avisa antes de sair com elas.
// Com o aviso de conflito na tela, o texto do usuário ainda não está no servidor.
export default class extends Controller {
  static targets = [ "source", "unsaved", "conflict" ]
  static values = { leaveMessage: String }

  connect() {
    this.original = this.sourceTarget.defaultValue
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

  warnBeforeUnload(event) {
    if (this.changed) event.preventDefault()
  }

  confirmVisit(event) {
    if (this.changed && !confirm(this.leaveMessageValue)) event.preventDefault()
  }

  get changed() {
    return !this.leaving && (this.hasConflictTarget || this.sourceTarget.value != this.original)
  }
}
