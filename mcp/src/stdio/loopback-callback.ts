import { createServer, type Server } from "node:http";

// LoopbackCallback é o servidor que recebe a volta do navegador: a redirect_uri dele e
// a query do primeiro retorno com o state esperado.
export type LoopbackCallback = {
  redirectUri: string;
  received: Promise<URLSearchParams>;
  close(): Promise<void>;
};

export type ListenForCallback = (
  expectedState: string,
) => Promise<LoopbackCallback>;

// CallbackReply é a resposta ao navegador e se o retorno encerra a espera.
export type CallbackReply = { status: number; html: string; accepted: boolean };

const CALLBACK_PATH = "/callback";

// callbackReply responde 400 ao state diferente e segue esperando, para uma aba velha
// ou uma requisição de outra origem não encerrarem o login.
export function callbackReply(url: URL, expectedState: string): CallbackReply {
  if (url.pathname !== CALLBACK_PATH) {
    return reply(404, "Rota desconhecida.", false);
  }
  if (url.searchParams.get("state") !== expectedState) {
    return reply(400, "Este retorno não é do login em andamento.", false);
  }
  if (url.searchParams.has("error")) {
    return reply(
      200,
      "Login recusado. Volte ao terminal para ver o motivo.",
      true,
    );
  }
  return reply(
    200,
    "Login concluído. Pode fechar esta aba e voltar ao terminal.",
    true,
  );
}

function reply(
  status: number,
  message: string,
  accepted: boolean,
): CallbackReply {
  const html = `<!doctype html><html lang="pt-BR"><meta charset="utf-8"><title>markupp-mcp</title><p>${message}</p></html>`;
  return { status, html, accepted };
}

// listenOnLoopback sobe o servidor em 127.0.0.1 numa porta livre: o Go aceita qualquer
// porta na redirect_uri em loopback por IP (RFC 8252, seção 7.3).
export function listenOnLoopback(
  expectedState: string,
): Promise<LoopbackCallback> {
  let deliver: (query: URLSearchParams) => void = () => undefined;
  const received = new Promise<URLSearchParams>((resolve) => {
    deliver = resolve;
  });
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? "/", "http://127.0.0.1");
    const answer = callbackReply(url, expectedState);
    response
      .writeHead(answer.status, {
        "content-type": "text/html; charset=utf-8",
        "cache-control": "no-store",
        connection: "close",
      })
      .end(answer.html);
    if (answer.accepted) deliver(url.searchParams);
  });
  return listening(server, received);
}

function listening(
  server: Server,
  received: Promise<URLSearchParams>,
): Promise<LoopbackCallback> {
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      const port = typeof address === "object" && address ? address.port : 0;
      resolve({
        redirectUri: `http://127.0.0.1:${port}${CALLBACK_PATH}`,
        received,
        close: () => closeServer(server),
      });
    });
  });
}

function closeServer(server: Server): Promise<void> {
  server.closeAllConnections();
  return new Promise((resolve) => server.close(() => resolve()));
}
