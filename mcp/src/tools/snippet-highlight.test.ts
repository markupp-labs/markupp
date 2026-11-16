import { describe, expect, it } from "vitest";
import { markdownHighlight } from "./snippet-highlight.js";

describe("markdownHighlight", () => {
  it("troca os marcadores de realce do Postgres por negrito markdown", () => {
    const snippet = "o \u0002agente\u0003 lê o \u0002agente\u0003";
    expect(markdownHighlight(snippet)).toBe("o **agente** lê o **agente**");
  });

  it("devolve o trecho sem marcadores como veio", () => {
    expect(markdownHighlight("nada realçado")).toBe("nada realçado");
  });
});
