import hljs from "highlight.js"
import { ON_DEMAND_GRAMMARS } from "editor/highlight_grammars"

// Apelido (ou nome) -> arquivo da gramática; num Map, "constructor" ou "__proto__" na cerca não acham nada.
const grammarFiles = new Map(Object.entries(ON_DEMAND_GRAMMARS).flatMap(([ file, aliases ]) => [ file, ...aliases ].map((alias) => [ alias, file ])))
const requestedGrammars = new Set()

// Destaque de sintaxe dos blocos de código cercados (opção highlight do markdown-it). Devolve só os spans
// hljs-*; o markdown-it põe o <pre><code> em volta e o DOMPurify limpa tudo depois. Linguagem desconhecida
// ou ausente devolve "", e o markdown-it mostra o código escapado, sem destaque. Linguagem fora do bundle também
// sai sem destaque desta vez, e a gramática dela é pedida (requestGrammar).
export function highlightCode(code, language) {
  if (!language) return ""
  if (!hljs.getLanguage(language)) return requestGrammar(language.toLowerCase())

  return hljs.highlight(code, { language, ignoreIllegals: true }).value
}

// Importa a gramática uma vez só (nem de novo se falhar) e avisa com highlight:grammar-loaded no document, para quem
// desenhou o markdown desenhar de novo, agora com destaque. Devolve "": o bloco desta vez fica sem destaque.
function requestGrammar(language) {
  const file = grammarFiles.get(language)
  if (!file || requestedGrammars.has(file)) return ""

  requestedGrammars.add(file)
  import(`highlight.js/languages/${file}`).then(({ default: grammar }) => {
    hljs.registerLanguage(file, grammar)
    document.dispatchEvent(new CustomEvent("highlight:grammar-loaded", { detail: { language: file } }))
  }, (error) => console.error(JSON.stringify({ event: "highlight_grammar_failed", language: file, error: String(error) })))
  return ""
}

// Nome da linguagem como o highlight.js a chama (js e javascript dão JavaScript); desconhecida fica como veio.
export const languageName = (language) => hljs.getLanguage(language)?.name ?? language
