import { UserError } from "fastmcp";
import { describe, expect, it } from "vitest";
import { TokenExchangeError } from "../auth/token-exchange.js";
import { MarkuppApiError } from "../markupp-api.js";
import { callApi, nextStepFor, toUserError } from "./tool-support.js";

describe("toUserError", () => {
  it("junta código, mensagem e o próximo passo do código conhecido", () => {
    const error = toUserError(
      new MarkuppApiError(409, "conflict", "nota foi atualizada"),
    );
    expect(error).toBeInstanceOf(UserError);
    expect(error.message).toMatch(
      /^conflict: nota foi atualizada\. Another write/,
    );
    expect(error.extras).toEqual({ code: "conflict", status: 409 });
  });

  it("deixa só código e mensagem no código sem próximo passo", () => {
    const error = toUserError(
      new MarkuppApiError(502, "unexpected_response", "corpo estranho"),
    );
    expect(error.message).toBe("unexpected_response: corpo estranho");
  });

  it.each([
    ["invalid_path", 400, /up to 1024 characters without '\.\.'/],
    ["invalid_content", 400, /Split the content into smaller notes/],
    ["insufficient_scope", 403, /granting the scope this message names/],
    ["forbidden", 403, /role owner or admin/],
    ["member_not_found", 404, /list_members/],
    ["invite_not_found", 404, /list_invites .*list_my_invites/],
    ["last_owner", 409, /owner role with change_member_role/],
    ["role_above_own", 403, /up to your own role/],
    ["already_member", 409, /list_members, or list_tenants/],
    ["unauthorized", 401, /Reconnect the server and sign in/],
    ["read_only", 403, /role owner, admin or member/],
    ["duplicate_path", 409, /^duplicate_path: x\. .*list_notes.*edit_note/],
  ])("dá o próximo passo de %s", (code, status, nextStep) => {
    const error = toUserError(new MarkuppApiError(status, code, "x"));
    expect(error.message).toMatch(nextStep);
  });
});

describe("nextStepFor", () => {
  it("ignora o que o mapa herda do Object", () => {
    expect(nextStepFor("toString")).toBeUndefined();
  });
});

describe("callApi", () => {
  it("devolve o resultado da chamada", async () => {
    expect(await callApi(async () => 42)).toBe(42);
  });

  it("deixa passar o erro que não é da API", async () => {
    const failure = callApi(async () => {
      throw new TypeError("rede caiu");
    });
    await expect(failure).rejects.toBeInstanceOf(TypeError);
  });

  it("deixa passar a falha de troca que não é do token do agente", async () => {
    const refused = new TokenExchangeError(401, "invalid_client", "segredo");
    const failure = callApi(async () => {
      throw refused;
    });
    await expect(failure).rejects.toBe(refused);
  });

  it("pede reconexão quando a renovação do JWT é recusada", async () => {
    const failure = callApi(async () => {
      throw new TokenExchangeError(400, "invalid_grant", "token revogado");
    });
    await expect(failure).rejects.toBeInstanceOf(UserError);
    await expect(failure).rejects.toThrow(
      /^unauthenticated: .*invalid_grant.*Reconnect the server and sign in\.$/,
    );
  });
});
