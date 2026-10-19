import { LanguageDescription, LanguageSupport, StreamLanguage } from "@codemirror/language"

// Linguagens dos blocos cercados no editor: as mesmas do highlight.js da leitura (highlight_code.js), com os apelidos
// dele. Cada uma só é importada quando um bloco dela aparece. css, html, javascript e markdown já vêm com o
// lang-markdown; as outras são modos das legacy-modes, fora do modulepreload (config/importmap.rb).
const streamMode = (mode) => new LanguageSupport(StreamLanguage.define(mode))

const LANGUAGES = [
  [ "bash", [ "sh", "zsh", "shell", "console", "shellsession" ], async () => streamMode((await import("@codemirror/legacy-modes/mode/shell")).shell) ],
  [ "css", [], async () => (await import("@codemirror/lang-css")).css() ],
  [ "diff", [ "patch" ], async () => streamMode((await import("@codemirror/legacy-modes/mode/diff")).diff) ],
  [ "go", [ "golang" ], async () => streamMode((await import("@codemirror/legacy-modes/mode/go")).go) ],
  [ "javascript", [ "js", "jsx", "mjs", "cjs" ], async () => (await import("@codemirror/lang-javascript")).javascript({ jsx: true }) ],
  [ "json", [ "jsonc", "json5" ], async () => streamMode((await import("@codemirror/legacy-modes/mode/javascript")).json) ],
  [ "markdown", [ "md", "mkdown", "mkd" ], async () => (await import("@codemirror/lang-markdown")).markdown() ],
  [ "python", [ "py", "gyp", "ipython" ], async () => streamMode((await import("@codemirror/legacy-modes/mode/python")).python) ],
  [ "ruby", [ "rb", "gemspec", "podspec", "thor", "irb" ], async () => streamMode((await import("@codemirror/legacy-modes/mode/ruby")).ruby) ],
  [ "sql", [], async () => streamMode((await import("@codemirror/legacy-modes/mode/sql")).standardSQL) ],
  [ "typescript", [ "ts", "tsx", "mts", "cts" ], async () => (await import("@codemirror/lang-javascript")).javascript({ typescript: true }) ],
  [ "xml", [ "html", "xhtml", "rss", "atom", "xjb", "xsd", "xsl", "plist", "wsf", "svg" ], async () => (await import("@codemirror/lang-html")).html() ],
  [ "yaml", [ "yml" ], async () => streamMode((await import("@codemirror/legacy-modes/mode/yaml")).yaml) ],
]

// Vai no codeLanguages do markdown(); nome sem par fica como texto do bloco, sem destaque.
export const codeLanguages = LANGUAGES.map(([ name, alias, load ]) => LanguageDescription.of({ name, alias, load }))
