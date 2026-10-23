#!/usr/bin/env node
import { spawn } from "node:child_process";
import { readFile } from "node:fs/promises";
import { homedir } from "node:os";
import { setTimeout as sleep } from "node:timers/promises";
import { runCli } from "./cli.js";
import { createHttpServer, startHttpServer } from "./http-server.js";
import { NodeLocalFiles } from "./stdio/local-files.js";
import { listenOnLoopback } from "./stdio/loopback-callback.js";
import { createBrowserOpener } from "./stdio/open-browser.js";
import { createStdioServer, startStdioServer } from "./stdio/stdio-server.js";

// Liga o CLI ao processo: stderr em texto para a pessoa, log em JSON também no stderr,
// e o stdout livre para o protocolo MCP do stdio.
process.exitCode = await runCli(process.argv.slice(2), process.env, {
  writeError: (line) => console.error(line),
  startup: {
    fetch,
    readFile: (path) => readFile(path, "utf8"),
    clock: Date.now,
    sleep: (ms) => sleep(ms),
  },
  local: {
    files: new NodeLocalFiles(),
    homeDir: homedir(),
    listenForCallback: listenOnLoopback,
    openBrowser: createBrowserOpener(process.platform, spawn),
    // O prazo do login não segura o processo depois que o callback chega.
    timeout: (ms) => sleep(ms, undefined, { ref: false }),
  },
  serveHttp: async (config) => {
    await startHttpServer(
      createHttpServer(config, {
        fetch,
        clock: Date.now,
        log: (entry) => console.error(JSON.stringify(entry)),
      }),
      config,
    );
  },
  serveStdio: (config) =>
    startStdioServer(
      createStdioServer(config, {
        fetch,
        writeLog: (line) => console.error(line),
      }),
    ),
});
