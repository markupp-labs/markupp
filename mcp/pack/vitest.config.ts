import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

// O teste do pacote fica fora da suíte unitária: empacota com npm pack, instala o
// tarball numa pasta descartável (o que baixa as dependências) e roda o binário pelo npx.
export default defineConfig({
  test: {
    root: fileURLToPath(new URL(".", import.meta.url)),
    include: ["**/*.pack.ts"],
    testTimeout: 30_000,
    hookTimeout: 300_000,
  },
});
