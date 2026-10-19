import { Controller } from "@hotwired/stimulus"

// Árvore de notas da barra lateral (notes/_folder.html.erb): a pasta abre e fecha o grupo dos filhos.
export default class extends Controller {
  toggle({ currentTarget }) {
    setExpanded(currentTarget, currentTarget.getAttribute("aria-expanded") != "true")
  }
}

function setExpanded(folder, expanded) {
  folder.setAttribute("aria-expanded", String(expanded))
  document.getElementById(folder.getAttribute("aria-owns")).hidden = !expanded
}
