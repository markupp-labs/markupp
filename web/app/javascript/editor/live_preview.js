import { Decoration, MatchDecorator, ViewPlugin } from "@codemirror/view"
import { syntaxTree } from "@codemirror/language"

// Live preview no estilo do Obsidian: esconde a sintaxe markdown fora das linhas com cursor.
// Só decora a tela; o documento (e o markdown salvo) continua intacto, byte a byte.
const hiddenSyntax = Decoration.replace({})

const insideInlineLink = (node) => node.parent?.name == "Link" && node.parent.getChild("URL") != null

const HIDDEN_MARKS = {
  HeaderMark: (node) => node.parent?.name.startsWith("ATXHeading"),
  EmphasisMark: () => true,
  CodeMark: (node) => node.parent?.name == "InlineCode",
  LinkMark: insideInlineLink,
  URL: insideInlineLink,
  LinkTitle: insideInlineLink,
}

function hiddenRange(state, node) {
  if (!HIDDEN_MARKS[node.name]?.(node)) return null

  const spaceAfterHeaderMark = node.name == "HeaderMark" && state.doc.sliceString(node.to, node.to + 1) == " "
  return { from: node.from, to: node.to + (spaceAfterHeaderMark ? 1 : 0) }
}

function cursorLineSpans(state) {
  return state.selection.ranges.map((range) => [ state.doc.lineAt(range.from).from, state.doc.lineAt(range.to).to ])
}

function hiddenRangesBetween(state, from, to) {
  const ranges = []
  syntaxTree(state).iterate({ from, to, enter: ({ node }) => { ranges.push(hiddenRange(state, node)) } })
  return ranges.filter(Boolean)
}

function hiddenSyntaxDecorations(view) {
  const spans = cursorLineSpans(view.state)
  const offCursorLine = ({ from, to }) => !spans.some(([ start, end ]) => from <= end && to >= start)
  const ranges = view.visibleRanges.flatMap(({ from, to }) => hiddenRangesBetween(view.state, from, to))
  return Decoration.set(ranges.filter(offCursorLine).map(({ from, to }) => hiddenSyntax.range(from, to)), true)
}

export const livePreview = ViewPlugin.fromClass(class {
  constructor(view) {
    this.decorations = hiddenSyntaxDecorations(view)
  }

  update(update) {
    const reparsed = syntaxTree(update.startState) != syntaxTree(update.state)
    if (update.docChanged || update.selectionSet || update.viewportChanged || reparsed) {
      this.decorations = hiddenSyntaxDecorations(update.view)
    }
  }
}, { decorations: (plugin) => plugin.decorations })

// [[wikilink]] só ganha destaque visual; não navega.
const wikilinkMatcher = new MatchDecorator({ regexp: /\[\[[^[\]\n]+\]\]/g, decoration: Decoration.mark({ class: "cm-wikilink" }) })

export const wikilinks = ViewPlugin.fromClass(class {
  constructor(view) {
    this.decorations = wikilinkMatcher.createDeco(view)
  }

  update(update) {
    this.decorations = wikilinkMatcher.updateDeco(update, this.decorations)
  }
}, { decorations: (plugin) => plugin.decorations })
