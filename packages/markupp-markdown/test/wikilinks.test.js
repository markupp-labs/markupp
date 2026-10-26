import { JSDOM } from "jsdom"
import { describe, expect, it } from "vitest"
import { createMarkdownRenderer, headingSlug, resolveWikilink } from "../dist/index.js"
import { FakeNoteDirectory } from "./fake_note_directory.js"

const render = createMarkdownRenderer(new JSDOM("").window)
const wikilinks = new FakeNoteDirectory({ Ideias: "/notes/1", "pasta/Plano": "/notes/2" })

describe("wikilinks", () => {
  it("vira link interno quando a nota existe", () => {
    expect(render("[[Ideias]]", { wikilinks })).toContain('<a href="/notes/1" class="wikilink">Ideias</a>')
  })

  it("usa o texto depois da barra", () => {
    expect(render("[[Ideias|minhas ideias]]", { wikilinks })).toContain('class="wikilink">minhas ideias</a>')
  })

  it("leva o título como âncora", () => {
    expect(render("[[Ideias#Próximos passos]]", { wikilinks })).toContain('href="/notes/1#proximos-passos"')
  })

  it("rola na própria nota com [[#título]]", () => {
    expect(render("[[#Resumo]]", { wikilinks })).toContain('<a href="#resumo" class="wikilink">#Resumo</a>')
  })

  it("marca o nome sem nota, com o motivo para o mouse e para o leitor de tela", () => {
    const html = render("[[Sumida]]", { wikilinks })

    expect(html).toContain('<span class="wikilink-missing" title="nota Sumida não existe">Sumida')
    expect(html).toContain('<span class="sr-only"> (nota Sumida não existe)</span>')
  })

  it("fica como texto sem env.wikilinks", () => {
    expect(render("[[Ideias]]")).toContain("<p>[[Ideias]]</p>")
  })
})

describe("resolveWikilink", () => {
  const targets = { "a/Plano": "/notes/a", "b/Plano": "/notes/b", "Plano.md": "/notes/raiz", "x/y/Longo": "/notes/longo" }

  it("prefere a nota da pasta atual", () => {
    expect(resolveWikilink("plano", { "a/Plano": "/notes/a", "b/Plano": "/notes/b" }, "b")).toBe("/notes/b")
  })

  it("sem a pasta atual, fica com o caminho mais curto", () => {
    expect(resolveWikilink("Longo.md", targets, "")).toBe("/notes/longo")
    expect(resolveWikilink("plano", { "a/Plano": "/notes/a", "b/Plano": "/notes/b" }, "")).toBe("/notes/a")
  })

  it("devolve undefined quando nenhum caminho casa", () => {
    expect(resolveWikilink("nada", targets, "")).toBeUndefined()
  })
})

describe("headingSlug", () => {
  it("tira acento e pontuação e troca espaço por hífen", () => {
    expect(headingSlug("  Ação, já! ")).toBe("acao-ja")
  })
})
