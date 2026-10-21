import type { SpawnOptions } from "node:child_process";
import { describe, expect, it } from "vitest";
import {
  browserCommand,
  createBrowserOpener,
  type SpawnedProcess,
} from "./open-browser.js";

const URL_WITH_QUERY = "http://localhost:8080/oauth/authorize?a=1&b=2";

// RecordingSpawn guarda cada processo que o abridor mandaria criar e quantos foram
// soltos do processo pai.
class RecordingSpawn {
  readonly calls: { command: string; args: string[]; options: SpawnOptions }[] =
    [];
  unrefs = 0;

  readonly spawn = (
    command: string,
    args: string[],
    options: SpawnOptions,
  ): SpawnedProcess => {
    this.calls.push({ command, args, options });
    return { once: () => undefined, unref: () => void this.unrefs++ };
  };
}

describe("browserCommand", () => {
  it("usa xdg-open no Linux e open no macOS", () => {
    expect(browserCommand("linux", URL_WITH_QUERY).command).toBe("xdg-open");
    expect(browserCommand("darwin", URL_WITH_QUERY).command).toBe("open");
  });

  it("protege o & da query no cmd do Windows", () => {
    expect(browserCommand("win32", URL_WITH_QUERY)).toEqual({
      command: "cmd",
      args: ["/c", "start", '""', `"${URL_WITH_QUERY}"`],
      verbatim: true,
    });
  });
});

describe("createBrowserOpener", () => {
  it("cria o processo solto do pai, sem herdar o stdio", () => {
    const recording = new RecordingSpawn();
    createBrowserOpener("linux", recording.spawn)(URL_WITH_QUERY);
    expect(recording.calls[0]?.args).toEqual([URL_WITH_QUERY]);
    expect(recording.calls[0]?.options).toMatchObject({
      detached: true,
      stdio: "ignore",
    });
    expect(recording.unrefs).toBe(1);
  });
});
