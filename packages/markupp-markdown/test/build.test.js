import { execFile } from "node:child_process"
import { readFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"
import { promisify } from "node:util"
import { describe, expect, it } from "vitest"
import { DIST_DIR, EDITOR_DIR, PACKAGED_EDITOR_MODULES, rewriteEditorImports } from "../scripts/build.js"

const PACKAGE_DIR = fileURLToPath(new URL("..", import.meta.url))
const REPOSITORY_LICENSE = new URL("../../../LICENSE", import.meta.url)
const PACKAGE_LICENSE = new URL("../LICENSE", import.meta.url)

async function listTarballFiles() {
  const { stdout } = await promisify(execFile)("npm", [ "pack", "--dry-run", "--json", "--ignore-scripts" ], { cwd: PACKAGE_DIR })
  return JSON.parse(stdout)[0].files.map((file) => file.path)
}

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

describe("licença do pacote", () => {
  it("o build copia o LICENSE AGPL da raiz do repositório para o pacote", async () => {
    expect(await readFile(PACKAGE_LICENSE, "utf8")).toBe(await readFile(REPOSITORY_LICENSE, "utf8"))
  })

  it("o tarball do npm leva o LICENSE", async () => {
    expect(await listTarballFiles()).toContain("LICENSE")
  }, 30000)
})
