// TEST_ISSUER é o markupp dos testes do stdio, em loopback como no self-host.
export const TEST_ISSUER = "http://localhost:8080";

// authServerMetadata é a metadata RFC 8414 que o Go publica para o issuer.
export function authServerMetadata(
  issuer: string = TEST_ISSUER,
): Record<string, string> {
  return {
    issuer,
    authorization_endpoint: `${issuer}/oauth/authorize`,
    token_endpoint: `${issuer}/oauth/token`,
    revocation_endpoint: `${issuer}/oauth/revoke`,
  };
}
