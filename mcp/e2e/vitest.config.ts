import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

// O e2e fica fora da suíte unitária: os arquivos terminam em .e2e.ts, que o padrão do
// Vitest não pega, e só esta config os inclui. Os arquivos rodam em série porque
// dividem a conta criada no primeiro login.
export default defineConfig({
  test: {
    root: fileURLToPath(new URL(".", import.meta.url)),
    include: ["**/*.e2e.ts"],
    globalSetup: ["./markupp-stack.ts"],
    fileParallelism: false,
    testTimeout: 60_000,
    hookTimeout: 120_000,
  },
});
