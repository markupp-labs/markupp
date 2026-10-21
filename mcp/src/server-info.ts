import { readFileSync } from "node:fs";

type SemanticVersion = `${number}.${number}.${number}`;

// readPackageVersion lê a versão do package.json, que fica um nível acima tanto de src
// nos testes quanto de dist no pacote publicado.
export function readPackageVersion(packageJsonUrl: URL): SemanticVersion {
  const { version } = JSON.parse(readFileSync(packageJsonUrl, "utf8")) as {
    version?: unknown;
  };
  if (typeof version !== "string" || !/^\d+\.\d+\.\d+/.test(version)) {
    throw new Error(
      `versao ${JSON.stringify(version)} em ${packageJsonUrl.pathname}, esperado x.y.z`,
    );
  }
  return version as SemanticVersion;
}

// SERVER_INFO é o nome e a versão que o servidor anuncia aos clientes MCP.
export const SERVER_INFO = {
  name: "markupp",
  version: readPackageVersion(new URL("../package.json", import.meta.url)),
};
