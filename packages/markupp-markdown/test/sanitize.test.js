import { JSDOM } from "jsdom"
import { describe, expect, it } from "vitest"
import { createMarkdownRenderer } from "../dist/index.js"

const render = createMarkdownRenderer(new JSDOM("").window)

describe("sanitização", () => {
  it("aceita html dentro do markdown", () => {
    expect(render("<mark>destaque</mark>")).toContain("<mark>destaque</mark>")
  })

  it("tira script", () => {
    expect(render("<script>alert(1)</script>texto")).not.toContain("script")
  })

  it("tira atributos on*", () => {
    expect(render('<img src="x.png" onerror="alert(1)">')).not.toContain("onerror")
  })

  it("tira href javascript:", () => {
    expect(render('<a href="javascript:alert(1)">clique</a>')).not.toContain("javascript:")
  })

  it("tira controles de formulário, que iriam junto com o form da nota", () => {
    const html = render('<form><input name="_method" value="delete"><button>ok</button></form>')

    expect(html).not.toMatch(/<(form|input|button)/)
  })

  it("prefixa id e name com user-content-", () => {
    expect(render('<a name="title">x</a>')).toContain('name="user-content-title"')
  })

  it("abre link externo em outra aba, sem acesso a esta", () => {
    expect(render("[site](https://example.com)")).toContain('target="_blank" rel="noopener noreferrer"')
  })

  it("mantém link relativo na mesma aba", () => {
    expect(render("[nota](/notes/1)")).not.toContain("target=")
  })
})
