# markupp-markdown

The markdown renderer of markupp notes: markdown-it with `[[wikilinks]]`, `![[note embeds]]`, heading ids,
syntax highlighting and DOMPurify sanitization. The same code renders notes in the markupp panel.

```js
import { JSDOM } from "jsdom"
import { createMarkdownRenderer, resolveWikilink } from "markupp-markdown"

const render = createMarkdownRenderer(new JSDOM("").window)
const targets = { "guides/Start here": "/docs/start-here" }
const wikilinks = {
  resolve: (name) => resolveWikilink(name, targets, "") ?? null,
  missingTitle: (name) => `${name} does not exist`,
}

render("See [[Start here]]", { wikilinks })
```

In the browser, `renderMarkdown(source, env)` uses the page window. In Node, pass a window from jsdom (a
dev dependency of yours) to `createMarkdownRenderer`: DOMPurify needs a DOM to sanitize, and the renderer
refuses a window without one.

## Development

The modules live in `web/app/javascript/editor` of this repository, where the panel loads them through its
importmap. `npm run build` copies them to `dist/`, rewriting the `editor/<module>` specifiers to relative
paths, and `npm test` builds before testing.
