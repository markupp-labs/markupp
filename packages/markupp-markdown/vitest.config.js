import { defineConfig } from "vitest/config"

// Os testes importam o dist/, gerado de novo antes de cada rodada a partir de web/app/javascript/editor.
export default defineConfig({
  test: {
    globalSetup: "./test/build_before_tests.js",
  },
})
