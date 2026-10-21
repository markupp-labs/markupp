import { describe, expect, it } from "vitest";
import { FakeFiles } from "./fake-files.js";

describe("FakeFiles", () => {
  it("devolve o conteúdo só depois das leituras que falham", async () => {
    const files = new FakeFiles().write("/s", "conteudo", 1);
    await expect(files.readFile("/s")).rejects.toThrow("ENOENT");
    expect(await files.readFile("/s")).toBe("conteudo");
    expect(files.reads).toEqual(["/s", "/s"]);
  });

  it("falha com o caminho do arquivo inexistente", async () => {
    await expect(new FakeFiles().readFile("/x")).rejects.toThrow("/x");
  });
});
