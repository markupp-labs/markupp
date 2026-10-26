import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

// PackageManifest é a parte do package.json que o npm mostra e usa ao publicar.
type PackageManifest = {
  name: string;
  bin: Record<string, string>;
  files: string[];
  engines: { node: string };
  homepage: string;
  keywords: string[];
  repository: { type: string; url: string; directory: string };
};

const manifest = JSON.parse(
  readFileSync(new URL("../package.json", import.meta.url), "utf8"),
) as PackageManifest;

describe("package.json publicado no npm", () => {
  it("expõe o binário markupp-mcp que o npx procura pelo nome do pacote", () => {
    expect(manifest.name).toBe("markupp-mcp");
    expect(manifest.bin).toEqual({ "markupp-mcp": "dist/bin.js" });
  });

  it("aponta o repositório que o trusted publishing confere", () => {
    expect(manifest.repository).toEqual({
      type: "git",
      url: "git+https://github.com/markupp-labs/markupp.git",
      directory: "mcp",
    });
    expect(manifest.homepage).toBe(
      "https://github.com/markupp-labs/markupp/tree/main/mcp#readme",
    );
  });

  it("aceita o Node 22 LTS, o piso das dependências", () => {
    expect(manifest.engines.node).toBe(">=22.12");
  });

  it("tem palavras-chave para a busca do npm", () => {
    expect(manifest.keywords).toEqual(
      expect.arrayContaining(["mcp", "model-context-protocol", "markupp"]),
    );
  });

  it("publica só o dist de produção, sem testes nem fakes", () => {
    expect(manifest.files).toEqual([
      "dist",
      "!dist/**/*.test.js",
      "!dist/testing",
    ]);
  });
});
