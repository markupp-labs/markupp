#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import { setTimeout as sleep } from "node:timers/promises";
import { runCli } from "./cli.js";
import { createHttpServer, startHttpServer } from "./http-server.js";

// Liga o CLI ao processo: stderr em texto para a pessoa, log de autenticação em JSON.
process.exitCode = await runCli(process.argv.slice(2), process.env, {
  writeError: (line) => console.error(line),
  startup: {
    fetch,
    readFile: (path) => readFile(path, "utf8"),
    clock: Date.now,
    sleep: (ms) => sleep(ms),
  },
  serveHttp: (config) =>
    startHttpServer(
      createHttpServer(config, {
        fetch,
        clock: Date.now,
        log: (entry) => console.error(JSON.stringify(entry)),
      }),
      config,
    ),
});
