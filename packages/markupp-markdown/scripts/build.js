import { copyFile, mkdir, readFile, rm, writeFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"

// O código do renderizador mora no painel (web/app/javascript/editor), que o carrega pelo importmap com
// especificadores "editor/<módulo>". O pacote copia esses módulos para dist/ trocando só esses especificadores por
// caminhos relativos: uma fonte só, e o dist/ não entra no git.
export const EDITOR_DIR = new URL("../../../web/app/javascript/editor/", import.meta.url)
export const DIST_DIR = new URL("../dist/", import.meta.url)
export const PACKAGED_EDITOR_MODULES = [ "render_markdown", "wikilinks", "note_embeds", "heading_ids", "highlight_code", "highlight_grammars", "code_blocks" ]

const ENTRY_POINT = new URL("../src/index.js", import.meta.url)
// A AGPL exige o texto da licença em cada cópia distribuída; o npm só põe no tarball o LICENSE da raiz do pacote.
const REPOSITORY_LICENSE = new URL("../../../LICENSE", import.meta.url)
const PACKAGE_LICENSE = new URL("../LICENSE", import.meta.url)
const EDITOR_SPECIFIER = /(["'])editor\/([^"']+)\1/g

export function rewriteEditorImports(source, fileName) {
  return source.replace(EDITOR_SPECIFIER, (_specifier, quote, name) => {
    if (!PACKAGED_EDITOR_MODULES.includes(name)) {
      throw new Error(`${fileName} importa editor/${name}, que não entra no pacote; esperado um de ${PACKAGED_EDITOR_MODULES.join(", ")}`)
    }
    return `${quote}./${name}.js${quote}`
  })
}

export async function buildPackage() {
  await rm(DIST_DIR, { recursive: true, force: true })
  await mkdir(DIST_DIR, { recursive: true })
  await Promise.all(PACKAGED_EDITOR_MODULES.map(copyEditorModule))
  await copyFile(ENTRY_POINT, new URL("index.js", DIST_DIR))
  await copyFile(REPOSITORY_LICENSE, PACKAGE_LICENSE)
}

async function copyEditorModule(name) {
  const source = await readFile(new URL(`${name}.js`, EDITOR_DIR), "utf8")
  await writeFile(new URL(`${name}.js`, DIST_DIR), rewriteEditorImports(source, `${name}.js`))
}

if (process.argv[1] === fileURLToPath(import.meta.url)) await buildPackage()
