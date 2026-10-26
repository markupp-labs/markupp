import { execFile } from "node:child_process";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

const run = promisify(execFile);
const PACKAGE_DIR = fileURLToPath(new URL("..", import.meta.url));
// STALE_BUILD é um .js que sobrou de um build antigo, como o de um módulo já apagado em src.
const STALE_BUILD = "dist/sobra-de-build-antigo.js";

// PackedTarball é o que o npm pack --json conta de cada tarball gerado.
type PackedTarball = { filename: string; files: { path: string }[] };

// BinOutcome é o código de saída e o stderr de uma execução do binário instalado.
type BinOutcome = { code: number; stderr: string };

let workdir = "";
let packed: PackedTarball = { filename: "", files: [] };

// packInto roda o npm pack de verdade, com o prepack, e devolve o relatório do tarball.
async function packInto(destination: string): Promise<PackedTarball> {
  const { stdout } = await run(
    "npm",
    ["pack", "--json", "--pack-destination", destination],
    { cwd: PACKAGE_DIR },
  );
  const [report] = JSON.parse(stdout.slice(stdout.indexOf("["))) as
    PackedTarball[] | [];
  if (report === undefined) {
    throw new Error(`npm pack sem tarball: ${stdout}, esperado um item`);
  }
  return report;
}

// leaveStaleBuild simula um dist sujo na máquina de quem publica à mão.
async function leaveStaleBuild(): Promise<void> {
  await mkdir(join(PACKAGE_DIR, "dist"), { recursive: true });
  await writeFile(join(PACKAGE_DIR, STALE_BUILD), "export {};\n");
}

// installTarball instala o tarball como faria quem roda npx markupp-mcp.
async function installTarball(dir: string, filename: string): Promise<void> {
  await writeFile(join(dir, "package.json"), '{"private":true}');
  await run(
    "npm",
    ["install", "--no-audit", "--no-fund", join(dir, filename)],
    { cwd: dir },
  );
}

// runPackedBin chama o binário pelo npx, com uma pasta pessoal sem login guardado.
async function runPackedBin(args: string[]): Promise<BinOutcome> {
  const env = {
    ...process.env,
    HOME: workdir,
    XDG_CONFIG_HOME: join(workdir, "config"),
  };
  try {
    const { stderr } = await run("npx", ["--no", "markupp-mcp", ...args], {
      cwd: workdir,
      env,
    });
    return { code: 0, stderr };
  } catch (error) {
    const failure = error as { code: number; stderr: string };
    return { code: failure.code, stderr: failure.stderr };
  }
}

beforeAll(async () => {
  workdir = await mkdtemp(join(tmpdir(), "markupp-mcp-pack-"));
  await leaveStaleBuild();
  packed = await packInto(workdir);
  await installTarball(workdir, packed.filename);
});

afterAll(async () => {
  await rm(workdir, { recursive: true, force: true });
  await rm(join(PACKAGE_DIR, STALE_BUILD), { force: true });
});

describe("tarball do npm", () => {
  it("leva o binário, o README e o package.json", () => {
    const paths = packed.files.map((file) => file.path);
    expect(paths).toEqual(
      expect.arrayContaining(["dist/bin.js", "README.md", "package.json"]),
    );
  });

  it("deixa de fora os testes e os fakes", () => {
    const leaked = packed.files
      .map((file) => file.path)
      .filter((path) => /\.test\.js$|^dist\/testing\//.test(path));
    expect(leaked).toEqual([]);
  });

  it("deixa de fora o que sobrou de um build antigo no dist", () => {
    const paths = packed.files.map((file) => file.path);
    expect(paths).not.toContain(STALE_BUILD);
  });
});

describe("npx markupp-mcp a partir do tarball", () => {
  it("mostra o uso num subcomando desconhecido", async () => {
    const outcome = await runPackedBin(["desconhecido"]);
    expect(outcome.code).toBe(2);
    expect(outcome.stderr).toContain("uso: markupp-mcp");
  });

  it("sobe o stdio e explica que falta o login", async () => {
    const outcome = await runPackedBin([]);
    expect(outcome.code).toBe(1);
    expect(outcome.stderr).toContain("rode markupp-mcp login");
  });
});
