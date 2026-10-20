import hljs from "highlight.js"

// Destaque de sintaxe dos blocos de código cercados (opção highlight do markdown-it). Devolve só os spans
// hljs-*; o markdown-it põe o <pre><code> em volta e o DOMPurify limpa tudo depois. Linguagem desconhecida
// ou ausente devolve "", e o markdown-it mostra o código escapado, sem destaque.
export function highlightCode(code, language) {
  if (!language || !hljs.getLanguage(language)) return ""

  return hljs.highlight(code, { language, ignoreIllegals: true }).value
}

// Nome da linguagem como o highlight.js a chama (js e javascript dão JavaScript); desconhecida fica como veio.
export const languageName = (language) => hljs.getLanguage(language)?.name ?? language
