import type { IncomingMessage } from "node:http";
import { describe, expect, it } from "vitest";
import { publicEndpointOf, publicSessionOf } from "./public-http-server.js";

// requestWith monta só o que publicSessionOf lê do pedido.
function requestWith(headers: IncomingMessage["headers"]): IncomingMessage {
  return { headers } as IncomingMessage;
}

describe("publicEndpointOf", () => {
  it("põe /public embaixo do endpoint autenticado", () => {
    expect(publicEndpointOf("/mcp")).toBe("/mcp/public");
    expect(publicEndpointOf("/agentes/mcp/")).toBe("/agentes/mcp/public");
  });
});

describe("publicSessionOf", () => {
  it("guarda o X-Forwarded-For recebido", () => {
    const request = requestWith({ "x-forwarded-for": "203.0.113.7, 10.0.0.2" });
    expect(publicSessionOf(request)).toEqual({
      forwardedFor: "203.0.113.7, 10.0.0.2",
    });
  });

  it("fica sem forwardedFor quando o header falta", () => {
    expect(publicSessionOf(requestWith({}))).toEqual({});
  });
});
