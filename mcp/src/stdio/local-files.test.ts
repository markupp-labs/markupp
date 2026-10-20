import { mkdtemp, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { NodeLocalFiles } from "./local-files.js";

// Estes testes usam um diretório temporário de verdade: NodeLocalFiles é a camada fina
// sobre o node:fs que os outros testes trocam por FakeLocalFiles.
describe("NodeLocalFiles", () => {
  const files = new NodeLocalFiles();
  let root = "";

  beforeEach(async () => {
    root = await mkdtemp(join(tmpdir(), "markupp-mcp-"));
  });

  afterEach(async () => {
    await rm(root, { recursive: true, force: true });
  });

  it("grava com diretório 0700 e arquivo 0600, sem sobrar temporário", async () => {
    const path = join(root, "markupp", "credentials.json");
    await files.writePrivate(path, "um");
    await files.writePrivate(path, "dois");
    expect(await files.readText(path)).toBe("dois");
    expect((await stat(path)).mode & 0o777).toBe(0o600);
    expect((await stat(join(root, "markupp"))).mode & 0o777).toBe(0o700);
    expect(await files.readText(`${path}.tmp`)).toBeUndefined();
  });

  it("devolve undefined para o arquivo inexistente", async () => {
    const path = join(root, "nada");
    expect(await files.readText(path)).toBeUndefined();
    expect(await files.modifiedAtMs(path)).toBeUndefined();
    await files.remove(path);
  });

  it("cria o lock uma vez só até ele ser apagado", async () => {
    const lock = join(root, "credentials.json.lock");
    expect(await files.createExclusive(lock)).toBe(true);
    expect(await files.createExclusive(lock)).toBe(false);
    expect(await files.modifiedAtMs(lock)).toBeTypeOf("number");
    await files.remove(lock);
    expect(await files.createExclusive(lock)).toBe(true);
  });
});
