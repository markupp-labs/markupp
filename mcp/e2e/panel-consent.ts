// Faz pela API o que a pessoa faz no painel: entra por senha e aprova o pedido de
// autorização que o authorize abriu.

// Com o banco vazio, o primeiro login cria a conta como owner do tenant inicial.
const ACCOUNT = { email: "e2e@markupp.local", password: "senha-do-e2e" };

async function expectStatus(
  response: Response,
  expected: number,
  what: string,
): Promise<void> {
  if (response.status === expected) return;
  const body = await response.text();
  throw new Error(
    `${what} respondeu ${response.status} ${body}, esperado ${expected}`,
  );
}

// loginByPassword devolve o access token da API da conta do e2e.
export async function loginByPassword(issuer: string): Promise<string> {
  const response = await fetch(`${issuer}/v1/auth/login`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(ACCOUNT),
  });
  await expectStatus(response, 200, "POST /v1/auth/login");
  const { access_token: token } = (await response.json()) as {
    access_token?: string;
  };
  if (!token) throw new Error("login sem access_token, esperado JSON com ele");
  return token;
}

// consentRequestId segue o authorize até a tela de consentimento do painel e lê o id
// do pedido, que vem no caminho /oauth/authorize/<id>.
async function consentRequestId(authorizeUrl: URL): Promise<string> {
  const response = await fetch(authorizeUrl, { redirect: "manual" });
  await expectStatus(response, 302, `GET ${authorizeUrl.pathname}`);
  const location = response.headers.get("location") ?? "";
  const path = new URL(location, authorizeUrl).pathname;
  const id = /^\/oauth\/authorize\/([^/]+)$/.exec(path)?.[1];
  if (!id) {
    throw new Error(
      `authorize redirecionou para ${location}, esperado o painel em /oauth/authorize/<id>`,
    );
  }
  return decodeURIComponent(id);
}

// approveInPanel aprova o pedido como a conta logada e devolve o redirect ao cliente,
// com o code e o state.
export async function approveInPanel(
  authorizeUrl: URL,
  apiToken: string,
): Promise<URL> {
  const id = await consentRequestId(authorizeUrl);
  const approve = `${authorizeUrl.origin}/v1/oauth/requests/${id}/approve`;
  const response = await fetch(approve, {
    method: "POST",
    headers: { authorization: `Bearer ${apiToken}` },
  });
  await expectStatus(response, 200, `POST ${approve}`);
  const { redirect_to: redirectTo } = (await response.json()) as {
    redirect_to?: string;
  };
  if (!redirectTo) throw new Error("aprovacao sem redirect_to, esperado URL");
  return new URL(redirectTo);
}
