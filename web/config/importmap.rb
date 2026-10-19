# Pin npm packages by running ./bin/importmap

pin "application"
pin "@hotwired/turbo-rails", to: "turbo.min.js"
pin "@hotwired/stimulus", to: "stimulus.min.js"
pin "@hotwired/stimulus-loading", to: "stimulus-loading.js"
pin_all_from "app/javascript/controllers", under: "controllers"
pin_all_from "app/javascript/editor", under: "editor"
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
# Preview de markdown; licencas de quem nao traz o aviso no arquivo em vendor/javascript/LICENSES.txt.
pin "dompurify" # @3.4.16
pin "markdown-it" # @15.0.2
pin "entities" # @8.1.0 (bundle do jsdelivr +esm, ver o cabecalho do arquivo)
pin "linkify-it" # @6.1.0
pin "mdurl" # @2.1.0
pin "punycode.js" # @2.3.1
pin "uc.micro" # @3.0.0
pin "highlight.js" # @11.12.0 (bundle com as linguagens comuns, ver o cabecalho do arquivo)
