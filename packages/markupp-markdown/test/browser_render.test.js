// @vitest-environment jsdom
import { describe, expect, it } from "vitest"
import { createMarkdownRenderer, renderMarkdown } from "../dist/index.js"
import { FakeNoteDirectory } from "./fake_note_directory.js"

// O caminho do painel: renderMarkdown sem window injetada, usando a global da página.
describe("renderMarkdown com a window global, como no painel", () => {
  const wikilinks = new FakeNoteDirectory({ Ideias: "/notes/1" })
  const source = "# Título\n\n[[Ideias]] e ![[Ideias]]\n\n<script>alert(1)</script>"

  it("renderiza igual ao renderizador com a window injetada", () => {
    expect(renderMarkdown(source, { wikilinks })).toBe(createMarkdownRenderer(window)(source, { wikilinks }))
  })

  it("sanitiza com a window da página", () => {
    expect(renderMarkdown(source, { wikilinks })).not.toContain("<script")
  })
})
