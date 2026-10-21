import { describe, expect, it } from "vitest";
import { countOccurrences, replaceExactlyOnce } from "./text-replacement.js";

describe("countOccurrences", () => {
  it("conta as ocorrências sobrepostas", () => {
    expect(countOccurrences("aaa", "aa")).toBe(2);
    expect(countOccurrences("abc", "x")).toBe(0);
  });
});

describe("replaceExactlyOnce", () => {
  it("troca o trecho único sem interpretar $ no texto novo", () => {
    expect(replaceExactlyOnce("preço: X", "X", "$& e $1")).toBe(
      "preço: $& e $1",
    );
  });

  it("recusa trecho ausente", () => {
    expect(() => replaceExactlyOnce("abc", "x", "y")).toThrow("occurs 0 times");
  });

  it("recusa trecho ambíguo com a contagem", () => {
    expect(() => replaceExactlyOnce("aaa", "aa", "b")).toThrow(
      "occurs 2 times",
    );
  });
});
