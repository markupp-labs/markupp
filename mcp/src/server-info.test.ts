import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { describe, expect, it } from "vitest";
import { SERVER_INFO, readPackageVersion } from "./server-info.js";

function packageJsonWith(content: unknown): URL {
  const path = join(
    mkdtempSync(join(tmpdir(), "markupp-mcp-")),
    "package.json",
  );
  writeFileSync(path, JSON.stringify(content));
  return pathToFileURL(path);
}

describe("readPackageVersion", () => {
  it("lê a versão do package.json", () => {
    expect(readPackageVersion(packageJsonWith({ version: "1.2.3" }))).toBe(
      "1.2.3",
    );
  });

  it("recusa versão fora de x.y.z citando o valor", () => {
    expect(() =>
      readPackageVersion(packageJsonWith({ version: "beta" })),
    ).toThrow('versao "beta"');
  });
});

describe("SERVER_INFO", () => {
  it("anuncia o markupp com a versão do pacote", () => {
    expect(SERVER_INFO.name).toBe("markupp");
    expect(SERVER_INFO.version).toMatch(/^\d+\.\d+\.\d+/);
  });
});
