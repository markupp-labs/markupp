import { Decoration, MatchDecorator, ViewPlugin } from "@codemirror/view"

// [[wikilink]] só ganha destaque visual no editor; navegar fica para a leitura.
const wikilinkMatcher = new MatchDecorator({ regexp: /\[\[[^[\]\n]+\]\]/g, decoration: Decoration.mark({ class: "cm-wikilink" }) })

export const wikilinkMarks = ViewPlugin.fromClass(class {
  constructor(view) {
    this.decorations = wikilinkMatcher.createDeco(view)
  }

  update(update) {
    this.decorations = wikilinkMatcher.updateDeco(update, this.decorations)
  }
}, { decorations: (plugin) => plugin.decorations })
