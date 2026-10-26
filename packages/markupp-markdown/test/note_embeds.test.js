import { JSDOM } from "jsdom"
import { afterEach, describe, expect, it, vi } from "vitest"
import { createMarkdownRenderer, embedContentLoader, fillEmbeds } from "../dist/index.js"
import { FakeNoteDirectory } from "./fake_note_directory.js"

const { window } = new JSDOM('<template id="spinner"><span class="spinner"></span></template><div id="preview"></div>')
const render = createMarkdownRenderer(window)
const wikilinks = new FakeNoteDirectory({ Ideias: "/notes/1" })

// GET /notes/:id.json do painel: url sem .json -> conteúdo; o resto responde 404.
class FakeNoteServer {
  constructor(contentByUrl) {
    this.contentByUrl = contentByUrl
    this.requests = []
  }

  fetch = async (url) => {
    this.requests.push(url)
    const content = this.contentByUrl[url.replace(/\.json$/, "")]
    if (content == null) return new Response("", { status: 404 })
    return Response.json({ content })
  }
}

const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

describe("embedRule", () => {
  it("monta o bloco da nota embutida com o link e o corpo vazio", () => {
    const html = render("![[Ideias]]", { wikilinks })

    expect(html).toContain('<span class="note-embed" data-embed-url="/notes/1">')
    expect(html).toContain('<a href="/notes/1" class="wikilink note-embed-link">Ideias</a>')
    expect(html).toContain('<span class="note-embed-body"></span>')
  })

  it("dentro de uma nota embutida vira link, sem embed dentro de embed", () => {
    const html = render("![[Ideias]]", { wikilinks, embedded: true })

    expect(html).not.toContain("note-embed")
    expect(html).toContain('<a href="/notes/1" class="wikilink">Ideias</a>')
  })

  it("nota que não existe vira o aviso de link quebrado", () => {
    expect(render("![[Sumida]]", { wikilinks })).toContain('class="wikilink-missing"')
  })
})

describe("fillEmbeds", () => {
  const preview = window.document.getElementById("preview")
  const spinner = window.document.getElementById("spinner")

  it("preenche o corpo com a nota renderizada como embutida", async () => {
    preview.innerHTML = render("![[Ideias]]", { wikilinks })
    const filled = []
    fillEmbeds(preview, async () => "# Dentro\n\n![[Ideias]]", { wikilinks }, { spinner, afterFill: (body) => filled.push(body), render })
    await settle()

    const body = preview.querySelector(".note-embed-body")
    expect(body.innerHTML).toContain("<h1>Dentro</h1>")
    expect(body.querySelector(".note-embed")).toBeNull()
    expect(filled).toEqual([ body ])
  })

  it("sem conteúdo deixa só o link, sem aria-busy", async () => {
    preview.innerHTML = render("![[Ideias]]", { wikilinks })
    fillEmbeds(preview, async () => null, { wikilinks }, { spinner, afterFill: () => {}, render })
    await settle()

    const body = preview.querySelector(".note-embed-body")
    expect(body.innerHTML).toBe("")
    expect(body.hasAttribute("aria-busy")).toBe(false)
  })
})

describe("embedContentLoader", () => {
  afterEach(() => vi.unstubAllGlobals())

  it("devolve o texto atual da própria nota sem buscar", async () => {
    const load = embedContentLoader({ noteUrls: [ "/notes/1" ], ownUrl: "/notes/1", ownContent: () => "não salvo" })

    expect(await load("/notes/1")).toBe("não salvo")
  })

  it("busca a nota da árvore uma vez só", async () => {
    const server = new FakeNoteServer({ "/notes/2": "corpo" })
    vi.stubGlobal("fetch", server.fetch)
    const load = embedContentLoader({ noteUrls: [ "/notes/2" ], ownUrl: "/notes/1", ownContent: () => "" })

    expect(await load("/notes/2")).toBe("corpo")
    expect(await load("/notes/2")).toBe("corpo")
    expect(server.requests).toEqual([ "/notes/2.json" ])
  })

  it("não busca url fora da árvore", async () => {
    const server = new FakeNoteServer({ "/externo": "x" })
    vi.stubGlobal("fetch", server.fetch)
    const load = embedContentLoader({ noteUrls: [], ownUrl: "/notes/1", ownContent: () => "" })

    expect(await load("/externo")).toBeNull()
    expect(server.requests).toEqual([])
  })

  it("devolve null quando a nota não responde json", async () => {
    vi.stubGlobal("fetch", new FakeNoteServer({}).fetch)
    const load = embedContentLoader({ noteUrls: [ "/notes/3" ], ownUrl: "/notes/1", ownContent: () => "" })

    expect(await load("/notes/3")).toBeNull()
  })
})
