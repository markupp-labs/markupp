import { JSDOM } from "jsdom"
import { describe, expect, it } from "vitest"
import { createMarkdownRenderer, renderMarkdown } from "../dist/index.js"

const render = createMarkdownRenderer(new JSDOM("").window)

describe("createMarkdownRenderer no Node", () => {
  it("renderiza markdown com a window do jsdom", () => {
    expect(render("**forte**")).toContain("<strong>forte</strong>")
  })

  it("dá id prefixado aos títulos, com sufixo no repetido", () => {
    const html = render("# Configuração inicial\n\n# Configuração inicial")

    expect(html).toContain('id="user-content-configuracao-inicial"')
    expect(html).toContain('id="user-content-configuracao-inicial-1"')
  })

  it("marca a linha do markdown de cada bloco", () => {
    expect(render("um\n\ndois")).toContain('<p data-source-line="2">dois</p>')
  })

  it("destaca o bloco de código e numera as linhas", () => {
    const html = render("```js\nconst a = 1\nconst b = 2\n```")

    expect(html).toContain('<span class="code-lines" aria-hidden="true">1\n2</span>')
    expect(html).toContain('class="hljs-keyword"')
  })

  it("recusa uma window sem DOM, que deixaria o html sem sanitizar", () => {
    expect(() => createMarkdownRenderer({})).toThrow(/window com document/)
  })
})

describe("renderMarkdown sem window global", () => {
  it("explica que no Node é preciso injetar a window", () => {
    expect(() => renderMarkdown("texto")).toThrow(/createMarkdownRenderer/)
  })
})
