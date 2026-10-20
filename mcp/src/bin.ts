#!/usr/bin/env node
import { runCli } from "./cli.js";
import { createHttpServer, startHttpServer } from "./http-server.js";

// Liga o CLI ao processo: stderr em texto para a pessoa, log de autenticação em JSON.
process.exitCode = await runCli(process.argv.slice(2), process.env, {
  writeError: (line) => console.error(line),
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
