import { describe, expect, it } from "vitest";
import { authServerMetadata, TEST_ISSUER } from "./auth-server-metadata.js";

describe("authServerMetadata", () => {
  it("põe os endpoints sob o issuer", () => {
    expect(authServerMetadata().token_endpoint).toBe(
      `${TEST_ISSUER}/oauth/token`,
    );
  });
});
