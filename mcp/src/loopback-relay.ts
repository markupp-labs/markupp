const LOOPBACK = "127.0.0.1";

// Headers de um salto só, que o fetch remonta para a conexão com o loopback.
const HOP_HEADERS = [
  "host",
  "connection",
  "keep-alive",
  "proxy-connection",
  "transfer-encoding",
  "te",
  "trailer",
  "upgrade",
  "content-length",
];

// O fetch já descomprime a resposta, então o content-encoding dela não vale mais.
const DECODED_RESPONSE_HEADERS = [...HOP_HEADERS, "content-encoding"];

// relayToLoopback repassa o pedido, com o mesmo caminho, query, corpo e headers, ao
// servidor na porta do loopback e devolve a resposta dele, em stream.
export async function relayToLoopback(
  request: Request,
  port: number,
): Promise<Response> {
  const { pathname, search } = new URL(request.url);
  const target = `http://${LOOPBACK}:${port}${pathname}${search}`;
  const upstream = await fetch(target, relayInit(request));
  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: without(upstream.headers, DECODED_RESPONSE_HEADERS),
  });
}

// StreamingRequestInit é o RequestInit com o duplex que o fetch do Node exige para
// mandar o corpo em stream.
type StreamingRequestInit = RequestInit & { duplex?: "half" };

function relayInit(request: Request): StreamingRequestInit {
  const init: StreamingRequestInit = {
    method: request.method,
    headers: without(request.headers, HOP_HEADERS),
    redirect: "manual",
  };
  if (request.method === "GET" || request.method === "HEAD") return init;
  return { ...init, body: request.body, duplex: "half" };
}

function without(headers: Headers, names: string[]): Headers {
  const copy = new Headers(headers);
  names.forEach((name) => copy.delete(name));
  return copy;
}
