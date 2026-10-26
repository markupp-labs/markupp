import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { agentClientsFor } from "./catalog.js";
import {
  EXAMPLE_INSTANCE_URL,
  buildClientsJson,
  serializeClientsJson,
  writeClientsJson,
} from "./clients-json.js";

// Cópia que o painel lê (web/config): sai do mesmo catálogo pelo npm run clients:sync.
const PANEL_COPY = new URL(
  "../../../web/config/agent_clients.json",
  import.meta.url,
);

describe("buildClientsJson", () => {
  it("leva os modelos com os marcadores em vez da url", () => {
    const json = buildClientsJson();
    expect(json.placeholders).toEqual({
      instanceUrl: "{{instanceUrl}}",
      mcpUrl: "{{mcpUrl}}",
    });
    expect(json.clients[0]?.snippets[0]?.template).toBe(
      "claude mcp add --transport http markupp {{mcpUrl}}",
    );
  });

  it("leva um exemplo já montado, que o painel usa de contrato", () => {
    const json = buildClientsJson();
    expect(json.example.instanceUrl).toBe(EXAMPLE_INSTANCE_URL);
    expect(json.example.clients).toEqual(agentClientsFor(EXAMPLE_INSTANCE_URL));
  });
});

describe("writeClientsJson", () => {
  it("grava o json serializado no caminho pedido", () => {
    const path = join(
      mkdtempSync(join(tmpdir(), "markupp-clients-")),
      "clients.json",
    );
    writeClientsJson(path);
    expect(readFileSync(path, "utf8")).toBe(serializeClientsJson());
  });
});

describe("cópia do painel", () => {
  it("é igual ao catálogo; rode npm run clients:sync se divergir", () => {
    expect(readFileSync(PANEL_COPY, "utf8")).toBe(serializeClientsJson());
  });
});
