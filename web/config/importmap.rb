# Pin npm packages by running ./bin/importmap

pin "application"
pin "@hotwired/turbo-rails", to: "turbo.min.js"
pin "@hotwired/stimulus", to: "stimulus.min.js"
pin "@hotwired/stimulus-loading", to: "stimulus-loading.js"
pin_all_from "app/javascript/controllers", under: "controllers"
pin_all_from "app/javascript/editor", under: "editor"
pin_all_from "app/javascript/motion", under: "motion"
pin "@codemirror/commands", to: "@codemirror--commands.js" # @6.11.1
pin "@codemirror/lang-markdown", to: "@codemirror--lang-markdown.js" # @6.5.2
pin "@codemirror/language", to: "@codemirror--language.js" # @6.12.4
pin "@codemirror/state", to: "@codemirror--state.js" # @6.7.6
pin "@codemirror/view", to: "@codemirror--view.js" # @6.43.13
pin "@codemirror/autocomplete", to: "@codemirror--autocomplete.js" # @6.20.3
pin "@codemirror/lang-css", to: "@codemirror--lang-css.js" # @6.3.1
pin "@codemirror/lang-html", to: "@codemirror--lang-html.js" # @6.4.12
pin "@codemirror/lang-javascript", to: "@codemirror--lang-javascript.js" # @6.2.5
pin "@lezer/common", to: "@lezer--common.js" # @1.5.3
pin "@lezer/css", to: "@lezer--css.js" # @1.3.8
pin "@lezer/highlight", to: "@lezer--highlight.js" # @1.2.5
pin "@lezer/html", to: "@lezer--html.js" # @1.3.13
pin "@lezer/javascript", to: "@lezer--javascript.js" # @1.5.6
pin "@lezer/lr", to: "@lezer--lr.js" # @1.4.10
pin "@lezer/markdown", to: "@lezer--markdown.js" # @1.7.2
pin "@marijn/find-cluster-break", to: "@marijn--find-cluster-break.js" # @1.0.3
pin "crelt" # @1.0.7
pin "style-mod" # @4.1.4
pin "w3c-keyname" # @2.2.8
# Linguagens dos blocos cercados no editor (editor/code_languages.js): sem preload, baixam quando aparece um bloco delas.
# Arquivos de mode/ do pacote npm sem mudança, salvo rust e dockerfile, que importam "./simple-mode.js" e aqui importam
# "@codemirror/legacy-modes/mode/simple-mode"; licença em vendor/javascript/LICENSES.txt.
pin "@codemirror/legacy-modes/mode/clike", to: "@codemirror--legacy-modes--mode--clike.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/clojure", to: "@codemirror--legacy-modes--mode--clojure.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/cmake", to: "@codemirror--legacy-modes--mode--cmake.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/coffeescript", to: "@codemirror--legacy-modes--mode--coffeescript.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/commonlisp", to: "@codemirror--legacy-modes--mode--commonlisp.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/crystal", to: "@codemirror--legacy-modes--mode--crystal.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/css", to: "@codemirror--legacy-modes--mode--css.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/d", to: "@codemirror--legacy-modes--mode--d.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/diff", to: "@codemirror--legacy-modes--mode--diff.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/dockerfile", to: "@codemirror--legacy-modes--mode--dockerfile.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/elm", to: "@codemirror--legacy-modes--mode--elm.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/erlang", to: "@codemirror--legacy-modes--mode--erlang.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/fortran", to: "@codemirror--legacy-modes--mode--fortran.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/gherkin", to: "@codemirror--legacy-modes--mode--gherkin.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/go", to: "@codemirror--legacy-modes--mode--go.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/groovy", to: "@codemirror--legacy-modes--mode--groovy.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/haskell", to: "@codemirror--legacy-modes--mode--haskell.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/http", to: "@codemirror--legacy-modes--mode--http.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/javascript", to: "@codemirror--legacy-modes--mode--javascript.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/julia", to: "@codemirror--legacy-modes--mode--julia.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/lua", to: "@codemirror--legacy-modes--mode--lua.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/mllike", to: "@codemirror--legacy-modes--mode--mllike.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/nginx", to: "@codemirror--legacy-modes--mode--nginx.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/octave", to: "@codemirror--legacy-modes--mode--octave.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/pascal", to: "@codemirror--legacy-modes--mode--pascal.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/perl", to: "@codemirror--legacy-modes--mode--perl.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/powershell", to: "@codemirror--legacy-modes--mode--powershell.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/properties", to: "@codemirror--legacy-modes--mode--properties.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/protobuf", to: "@codemirror--legacy-modes--mode--protobuf.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/python", to: "@codemirror--legacy-modes--mode--python.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/r", to: "@codemirror--legacy-modes--mode--r.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/ruby", to: "@codemirror--legacy-modes--mode--ruby.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/rust", to: "@codemirror--legacy-modes--mode--rust.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/scheme", to: "@codemirror--legacy-modes--mode--scheme.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/shell", to: "@codemirror--legacy-modes--mode--shell.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/simple-mode", to: "@codemirror--legacy-modes--mode--simple-mode.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/sql", to: "@codemirror--legacy-modes--mode--sql.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/stex", to: "@codemirror--legacy-modes--mode--stex.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/swift", to: "@codemirror--legacy-modes--mode--swift.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/tcl", to: "@codemirror--legacy-modes--mode--tcl.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/toml", to: "@codemirror--legacy-modes--mode--toml.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/vb", to: "@codemirror--legacy-modes--mode--vb.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/verilog", to: "@codemirror--legacy-modes--mode--verilog.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/vhdl", to: "@codemirror--legacy-modes--mode--vhdl.js", preload: false # @6.5.4
pin "@codemirror/legacy-modes/mode/yaml", to: "@codemirror--legacy-modes--mode--yaml.js", preload: false # @6.5.4
# PHP: lang-php e o parser lezer dele baixados do ga.jspm.io (cabeçalho de cada arquivo), sem preload como os modos acima.
pin "@codemirror/lang-php", to: "@codemirror--lang-php.js", preload: false # @6.0.2
pin "@lezer/php", to: "@lezer--php.js", preload: false # @1.0.6
# Preview de markdown; licencas de quem nao traz o aviso no arquivo em vendor/javascript/LICENSES.txt.
pin "dompurify" # @3.4.17
pin "markdown-it" # @15.0.3
pin "entities" # @8.1.0 (bundle do jsdelivr +esm, ver o cabecalho do arquivo)
pin "linkify-it" # @6.1.0
pin "mdurl" # @2.1.0
pin "punycode.js" # @2.3.1
pin "uc.micro" # @3.0.0
pin "highlight.js" # @11.12.0 (bundle com as linguagens comuns, ver o cabecalho do arquivo)
# Gramaticas do highlight.js fora do bundle acima, uma por arquivo (es/languages/<nome>.min.js de @highlightjs/cdn-assets@11.12.0,
# sem mudanca, renomeado para <nome>.js): sem preload, editor/highlight_code.js importa a de um bloco quando ele aparece.
pin_all_from "vendor/javascript/highlightjs-languages", under: "highlight.js/languages", to: "highlightjs-languages", preload: false
# Modo Vim do editor (editor/vim_mode.js): sem preload, baixa so com o modo ligado nas configuracoes. Arquivos do
# ga.jspm.io (cabecalho de cada um); o search entra como dependencia do vim. Licencas em vendor/javascript/LICENSES.txt.
pin "@replit/codemirror-vim", to: "@replit--codemirror-vim.js", preload: false # @6.4.0
pin "@replit/codemirror-vim-core", to: "@replit--codemirror-vim-core.js", preload: false # @0.1.0
pin "@codemirror/search", to: "@codemirror--search.js", preload: false # @6.7.2
# Grafo de forca (graph/force_layout.js): sem preload, baixa so quando a visao Graph ou a tela do grafo abre. Arquivos do
# ga.jspm.io sem mudanca (cabecalho de cada um); licenca ISC em vendor/javascript/LICENSES.txt.
pin "d3-force", preload: false # @3.0.0
pin "d3-dispatch", preload: false # @3.0.1
pin "d3-quadtree", preload: false # @3.0.1
pin "d3-timer", preload: false # @3.0.1
# Desenho do grafo (graph_view_controller.js importa sob demanda): sem preload, como o d3-force.
pin_all_from "app/javascript/graph", under: "graph", preload: false
