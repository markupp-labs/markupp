import { Controller } from "@hotwired/stimulus"
import { Turbo } from "@hotwired/turbo-rails"
import { isTyping } from "controllers/typing"

// g e depois t, fora de campo de texto, abre a lista de tenants; g e depois v, o seletor de cofres da trilha, com o foco
// no primeiro cofre (context_menu_controller.js). v e depois r, a ou q age no cofre ativo: renomear, adicionar membro e
// sair. Com a ação na página (data-vault-action), foca o campo ou aperta o botão; sem ela, abre a página da ação
// (vaultActions, de ShortcutsHelper#vault_shortcut_paths), que chega com o foco nela. Atalho em sequência como o gt do
// Vim e o g n do GitHub, sem disputar Ctrl ou Alt com o navegador. Qualquer outra tecla no meio desfaz o prefixo.
const PREFIXES = [ "g", "v" ]

export default class extends Controller {
  static values = { url: String, vaultActions: Object }

  open(event) {
    const prefix = this.prefix
    this.prefix = null
    if (event.ctrlKey || event.metaKey || event.altKey || event.defaultPrevented || isTyping(event.target)) return
    if (prefix == "g" && event.key == "t") {
      event.preventDefault()
      return Turbo.visit(this.urlValue)
    }
    if (prefix == "g" && event.key == "v") return this.openVaults(event)
    if (prefix == "v" && event.key in this.vaultActionsValue) return this.runVaultAction(event)
    if (PREFIXES.includes(event.key)) this.prefix = event.key
  }

  // O click de teclado (detail 0) leva o foco ao primeiro item, como o Enter no nome do cofre.
  openVaults(event) {
    const opener = document.querySelector("button[popovertarget=vault_switcher]")
    if (!opener) return
    event.preventDefault()
    opener.click()
  }

  // O Leave vault pede a confirmação dele (data-turbo-confirm); o nome e a escolha do colega só recebem o foco.
  runVaultAction(event) {
    event.preventDefault()
    const target = document.querySelector(`[data-vault-action="${event.key}"]`)
    if (!target) return Turbo.visit(this.vaultActionsValue[event.key])

    target.matches("button") ? target.click() : target.focus()
  }
}
