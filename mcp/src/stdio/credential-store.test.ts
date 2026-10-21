import { describe, expect, it } from "vitest";
import { FakeClock } from "../testing/fake-clock.js";
import { FakeLocalFiles } from "../testing/fake-local-files.js";
import { CredentialStore, credentialsPath } from "./credential-store.js";

const PATH = "/home/ana/.config/markupp/credentials.json";
const CREDENTIAL = {
  issuer: "http://localhost:8080",
  refreshToken: "r1",
  accessToken: "jwt",
  expiresAtMs: Date.UTC(2026, 9, 20, 12, 5),
};

describe("credentialsPath", () => {
  it("usa o XDG_CONFIG_HOME absoluto", () => {
    expect(credentialsPath({ XDG_CONFIG_HOME: "/x" }, "/home/ana")).toBe(
      "/x/markupp/credentials.json",
    );
  });

  it("cai no ~/.config sem XDG_CONFIG_HOME ou com ele relativo", () => {
    expect(credentialsPath({}, "/home/ana")).toBe(PATH);
    expect(credentialsPath({ XDG_CONFIG_HOME: "rel" }, "/home/ana")).toBe(PATH);
  });
});

describe("CredentialStore", () => {
  it("grava pela escrita privada e lê de volta", async () => {
    const files = new FakeLocalFiles(new FakeClock().now);
    const store = new CredentialStore(PATH, files);
    await store.save(CREDENTIAL);
    expect(files.privateWrites).toEqual([PATH]);
    expect(await store.load()).toEqual(CREDENTIAL);
    expect(JSON.parse((await files.readText(PATH)) ?? "")).toEqual({
      issuer: "http://localhost:8080",
      refresh_token: "r1",
      access_token: "jwt",
      expires_at: "2026-10-20T12:05:00.000Z",
    });
  });

  it("devolve undefined sem arquivo e apaga o que existe", async () => {
    const files = new FakeLocalFiles(new FakeClock().now);
    const store = new CredentialStore(PATH, files);
    expect(await store.load()).toBeUndefined();
    await store.save(CREDENTIAL);
    await store.remove();
    expect(await store.load()).toBeUndefined();
  });

  it("recusa o arquivo inválido citando os campos, nunca o token", async () => {
    const files = new FakeLocalFiles(new FakeClock().now);
    files.put(PATH, JSON.stringify({ issuer: "x", refresh_token: "segredo" }));
    const store = new CredentialStore(PATH, files);
    const failure = await store.load().catch((error: unknown) => String(error));
    expect(failure).toContain(`credencial em ${PATH} invalida`);
    expect(failure).toContain("access_token, expires_at");
    expect(failure).not.toContain("segredo");
  });

  it("põe o lock ao lado da credencial", () => {
    const store = new CredentialStore(PATH, new FakeLocalFiles(Date.now));
    expect(store.lockPath).toBe(`${PATH}.lock`);
  });
});
