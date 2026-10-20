import { LanguageDescription, LanguageSupport, StreamLanguage } from "@codemirror/language"

// Linguagens dos blocos cercados no editor: as mesmas do highlight.js da leitura (highlight_code.js), com os apelidos
// dele. Cada uma só é importada quando um bloco dela aparece. css, html, javascript e markdown já vêm com o
// lang-markdown; as outras são modos das legacy-modes, fora do modulepreload (config/importmap.rb).
const streamMode = (mode) => new LanguageSupport(StreamLanguage.define(mode))
// Carregador do modo exportado como mode no arquivo mode/<file> das legacy-modes.
const legacyMode = (file, mode) => async () => streamMode((await import(`@codemirror/legacy-modes/mode/${file}`))[mode])

const LANGUAGES = [
  [ "bash", [ "sh", "zsh", "shell", "console", "shellsession" ], legacyMode("shell", "shell") ],
  [ "css", [], async () => (await import("@codemirror/lang-css")).css() ],
  [ "diff", [ "patch" ], legacyMode("diff", "diff") ],
  [ "go", [ "golang" ], legacyMode("go", "go") ],
  [ "javascript", [ "js", "jsx", "mjs", "cjs" ], async () => (await import("@codemirror/lang-javascript")).javascript({ jsx: true }) ],
  [ "json", [ "jsonc", "json5" ], legacyMode("javascript", "json") ],
  [ "markdown", [ "md", "mkdown", "mkd" ], async () => (await import("@codemirror/lang-markdown")).markdown() ],
  [ "python", [ "py", "gyp", "ipython" ], legacyMode("python", "python") ],
  [ "ruby", [ "rb", "gemspec", "podspec", "thor", "irb" ], legacyMode("ruby", "ruby") ],
  [ "sql", [], legacyMode("sql", "standardSQL") ],
  [ "typescript", [ "ts", "tsx", "mts", "cts" ], async () => (await import("@codemirror/lang-javascript")).javascript({ typescript: true }) ],
  [ "xml", [ "html", "xhtml", "rss", "atom", "xjb", "xsd", "xsl", "plist", "wsf", "svg" ], async () => (await import("@codemirror/lang-html")).html() ],
  [ "yaml", [ "yml" ], legacyMode("yaml", "yaml") ],
]

// Vai no codeLanguages do markdown(); nome sem par fica como texto do bloco, sem destaque.
export const codeLanguages = LANGUAGES.map(([ name, alias, load ]) => LanguageDescription.of({ name, alias, load }))
