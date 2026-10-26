import { readFile } from "node:fs/promises"
import { describe, expect, it } from "vitest"
import { DIST_DIR, EDITOR_DIR, PACKAGED_EDITOR_MODULES, rewriteEditorImports } from "../scripts/build.js"

const readModule = (dir, name) => readFile(new URL(`${name}.js`, dir), "utf8")

describe("rewriteEditorImports", () => {
  it("troca o especificador do importmap pelo caminho relativo", () => {
    expect(rewriteEditorImports('import { headingSlug } from "editor/heading_ids"', "wikilinks.js")).toBe('import { headingSlug } from "./heading_ids.js"')
  })

  it("recusa módulo do editor que não entra no pacote, com o nome do arquivo", () => {
    expect(() => rewriteEditorImports('import { vim } from "editor/vim_mode"', "render_markdown.js")).toThrow(/render_markdown\.js importa editor\/vim_mode/)
  })
})

describe("dist gerado do editor do painel", () => {
  it.each(PACKAGED_EDITOR_MODULES)("%s.js é a cópia do painel, só com os imports reescritos", async (name) => {
    const source = await readModule(EDITOR_DIR, name)

    expect(await readModule(DIST_DIR, name)).toBe(rewriteEditorImports(source, `${name}.js`))
  })

  it.each(PACKAGED_EDITOR_MODULES)("%s.js não sobra com especificador do importmap", async (name) => {
    expect(await readModule(DIST_DIR, name)).not.toMatch(/["']editor\//)
  })

  it("a entrada do package.json existe no dist", async () => {
    const manifest = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"))

    await expect(readFile(new URL(`../${manifest.exports["."]}`, import.meta.url), "utf8")).resolves.toContain("createMarkdownRenderer")
  })
})
