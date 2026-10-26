import { writeClientsJson } from "./clients-json.js";

// Uso: node dist/clients/export-clients-json.js <caminho>. O build grava dist/clients.json
// e o npm run clients:sync grava a cópia do painel.
const [path, ...extra] = process.argv.slice(2);
if (path === undefined || extra.length > 0) {
  process.stderr.write(
    `argumentos ${JSON.stringify(process.argv.slice(2))}, esperado um caminho de saida\n`,
  );
  process.exit(2);
}
writeClientsJson(path);
