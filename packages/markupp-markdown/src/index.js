// Entrada do pacote: os módulos ao lado são cópias dos de web/app/javascript/editor (scripts/build.js).
// No Node, createMarkdownRenderer(new JSDOM("").window); no navegador, renderMarkdown usa a window da página.
export { createMarkdownRenderer, renderMarkdown } from "./render_markdown.js"
export { resolveWikilink } from "./wikilinks.js"
export { headingSlug, scrollToHeading } from "./heading_ids.js"
export { embedContentLoader, fillEmbeds } from "./note_embeds.js"
export { addCodeTools, copyCode } from "./code_blocks.js"
