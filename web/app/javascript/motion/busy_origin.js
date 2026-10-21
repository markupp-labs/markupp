import { Turbo } from "@hotwired/turbo-rails"

// Espera no lugar da ação, como no VS Code: o link ou botão que começou a visita ou o envio ganha aria-busy na hora e,
// no fim dele, o busy_spinner do template busy_origin_spinner (BusySpinnerHelper), que só mostra o glifo braille depois
// de 300ms. O Turbo diz de onde veio: o link no turbo:click, o botão no turbo:submit-start; envio sem botão (Ctrl+S)
// usa o botão que o envio teria usado. Ação por código (paleta, menu da árvore) diz a origem com visitFrom, submitFrom e rememberSubmitOrigin.
// A espera sai no turbo:submit-end, no turbo:load e antes de o Turbo guardar a página no cache (voltar não a traz).
const TEMPLATE_ID = "busy_origin_spinner"
const submitOrigins = new WeakMap()
let visitOrigin = null
let busyElement = null

// Visita com origem; diz se ela começou (a nota com mudança pode perguntar e o usuário ficar). O turbo:visit sai
// dentro do Turbo.visit, antes de ele voltar, e é ele que consome a origem.
export function visitFrom(origin, url) {
  visitOrigin = origin
  Turbo.visit(url)
  const started = visitOrigin == null
  visitOrigin = null
  return started
}

export function submitFrom(origin, form) {
  rememberSubmitOrigin(form, origin)
  form.requestSubmit()
}

// Envio por um botão que não fica à vista (o item do menu, que fecha): a espera vai para a origem dada.
export function rememberSubmitOrigin(form, origin) {
  submitOrigins.set(form, origin)
}

function markBusy(origin) {
  clearBusy()
  const template = document.getElementById(TEMPLATE_ID)
  if (!origin?.isConnected || !template) return

  busyElement = origin
  origin.setAttribute("aria-busy", "true")
  origin.append(template.content.cloneNode(true))
}

function clearBusy() {
  if (!busyElement) return

  busyElement.removeAttribute("aria-busy")
  busyElement.querySelector(":scope > .busy-spinner-origin")?.remove()
  busyElement = null
}

// O clique no link vem logo antes da visita, na mesma volta; visita cancelada não deixa a origem para a próxima.
function rememberClickedLink({ target }) {
  visitOrigin = target
  queueMicrotask(() => (visitOrigin = null))
}

function markVisitOrigin() {
  markBusy(visitOrigin)
  visitOrigin = null
}

function markSubmitOrigin({ target: form, detail: { formSubmission } }) {
  const origin = submitOrigins.get(form) ?? formSubmission.submitter ?? defaultButton(form)
  submitOrigins.delete(form)
  markBusy(origin)
}

// O botão que o Enter usaria: o primeiro de submit do form, se estiver à vista.
function defaultButton(form) {
  const button = [ ...form.elements ].find((element) => element.type == "submit")
  return button?.checkVisibility() ? button : null
}

document.addEventListener("turbo:click", rememberClickedLink)
document.addEventListener("turbo:visit", markVisitOrigin)
document.addEventListener("turbo:submit-start", markSubmitOrigin)
for (const done of [ "turbo:submit-end", "turbo:load", "turbo:before-cache" ]) document.addEventListener(done, clearBusy)
