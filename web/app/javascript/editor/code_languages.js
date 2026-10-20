import { LanguageDescription, LanguageSupport, StreamLanguage } from "@codemirror/language"

// Linguagens dos blocos cercados no editor: as comuns da leitura (highlight_code.js) que têm modo no CodeMirror 6, com
// os nomes e apelidos do highlight.js. Cada uma só é importada quando um bloco dela aparece. css, html, javascript e
// markdown já vêm com o lang-markdown; php vem do lang-php; as outras são modos das legacy-modes. Tudo fora do
// modulepreload (config/importmap.rb). toml tem modo próprio aqui, embora na leitura seja apelido de ini.
const streamMode = (mode) => new LanguageSupport(StreamLanguage.define(mode))
// Carregador do modo exportado como mode no arquivo mode/<file> das legacy-modes.
const legacyMode = (file, mode) => async () => streamMode((await import(`@codemirror/legacy-modes/mode/${file}`))[mode])

const LANGUAGES = [
  [ "bash", [ "sh", "zsh", "shell", "console", "shellsession" ], legacyMode("shell", "shell") ],
  [ "c", [ "h" ], legacyMode("clike", "c") ],
  [ "clojure", [ "clj", "edn" ], legacyMode("clojure", "clojure") ],
  [ "cmake", [ "cmake.in" ], legacyMode("cmake", "cmake") ],
  [ "coffeescript", [ "coffee", "cson", "iced" ], legacyMode("coffeescript", "coffeeScript") ],
  [ "cpp", [ "cc", "c++", "h++", "hpp", "hh", "hxx", "cxx" ], legacyMode("clike", "cpp") ],
  [ "crystal", [ "cr" ], legacyMode("crystal", "crystal") ],
  [ "csharp", [ "cs", "c#" ], legacyMode("clike", "csharp") ],
  [ "css", [], async () => (await import("@codemirror/lang-css")).css() ],
  [ "d", [], legacyMode("d", "d") ],
  [ "dart", [], legacyMode("clike", "dart") ],
  [ "delphi", [ "dpr", "dfm", "pas", "pascal" ], legacyMode("pascal", "pascal") ],
  [ "diff", [ "patch" ], legacyMode("diff", "diff") ],
  [ "dockerfile", [ "docker" ], legacyMode("dockerfile", "dockerFile") ],
  [ "elm", [], legacyMode("elm", "elm") ],
  [ "erlang", [ "erl" ], legacyMode("erlang", "erlang") ],
  [ "fortran", [ "f90", "f95" ], legacyMode("fortran", "fortran") ],
  [ "fsharp", [ "fs", "f#" ], legacyMode("mllike", "fSharp") ],
  [ "gherkin", [ "feature" ], legacyMode("gherkin", "gherkin") ],
  [ "go", [ "golang" ], legacyMode("go", "go") ],
  [ "gradle", [], legacyMode("groovy", "groovy") ],
  [ "groovy", [], legacyMode("groovy", "groovy") ],
  [ "haskell", [ "hs" ], legacyMode("haskell", "haskell") ],
  [ "http", [ "https" ], legacyMode("http", "http") ],
  [ "ini", [], legacyMode("properties", "properties") ],
  [ "java", [ "jsp" ], legacyMode("clike", "java") ],
  [ "javascript", [ "js", "jsx", "mjs", "cjs" ], async () => (await import("@codemirror/lang-javascript")).javascript({ jsx: true }) ],
  [ "json", [ "jsonc", "json5" ], legacyMode("javascript", "json") ],
  [ "julia", [], legacyMode("julia", "julia") ],
  [ "kotlin", [ "kt", "kts", "ktm", "ktx" ], legacyMode("clike", "kotlin") ],
  [ "latex", [ "tex" ], legacyMode("stex", "stex") ],
  [ "less", [], legacyMode("css", "less") ],
  [ "lisp", [], legacyMode("commonlisp", "commonLisp") ],
  [ "lua", [ "pluto" ], legacyMode("lua", "lua") ],
  [ "markdown", [ "md", "mkdown", "mkd" ], async () => (await import("@codemirror/lang-markdown")).markdown() ],
  [ "matlab", [], legacyMode("octave", "octave") ],
  [ "nginx", [ "nginxconf" ], legacyMode("nginx", "nginx") ],
  [ "objectivec", [ "mm", "objc", "obj-c", "obj-c++", "objective-c++" ], legacyMode("clike", "objectiveC") ],
  [ "ocaml", [ "ml" ], legacyMode("mllike", "oCaml") ],
  [ "perl", [ "pl", "pm" ], legacyMode("perl", "perl") ],
  [ "php", [], async () => (await import("@codemirror/lang-php")).php() ],
  [ "powershell", [ "pwsh", "ps", "ps1" ], legacyMode("powershell", "powerShell") ],
  [ "properties", [], legacyMode("properties", "properties") ],
  [ "protobuf", [ "proto" ], legacyMode("protobuf", "protobuf") ],
  [ "python", [ "py", "gyp", "ipython" ], legacyMode("python", "python") ],
  [ "r", [], legacyMode("r", "r") ],
  [ "ruby", [ "rb", "gemspec", "podspec", "thor", "irb" ], legacyMode("ruby", "ruby") ],
  [ "rust", [ "rs" ], legacyMode("rust", "rust") ],
  [ "scala", [], legacyMode("clike", "scala") ],
  [ "scheme", [ "scm" ], legacyMode("scheme", "scheme") ],
  [ "scss", [], legacyMode("css", "sCSS") ],
  [ "sml", [], legacyMode("mllike", "sml") ],
  [ "sql", [], legacyMode("sql", "standardSQL") ],
  [ "swift", [], legacyMode("swift", "swift") ],
  [ "tcl", [ "tk" ], legacyMode("tcl", "tcl") ],
  [ "toml", [], legacyMode("toml", "toml") ],
  [ "typescript", [ "ts", "tsx", "mts", "cts" ], async () => (await import("@codemirror/lang-javascript")).javascript({ typescript: true }) ],
  [ "vbnet", [ "vb" ], legacyMode("vb", "vb") ],
  [ "verilog", [ "v", "sv", "svh" ], legacyMode("verilog", "verilog") ],
  [ "vhdl", [], legacyMode("vhdl", "vhdl") ],
  [ "xml", [ "html", "xhtml", "rss", "atom", "xjb", "xsd", "xsl", "plist", "wsf", "svg" ], async () => (await import("@codemirror/lang-html")).html() ],
  [ "yaml", [ "yml" ], legacyMode("yaml", "yaml") ],
]

// Vai no codeLanguages do markdown(); nome sem par fica como texto do bloco, sem destaque.
export const codeLanguages = LANGUAGES.map(([ name, alias, load ]) => LanguageDescription.of({ name, alias, load }))
