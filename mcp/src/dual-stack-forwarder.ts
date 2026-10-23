import { connect, createServer, type Socket } from "node:net";

const LOOPBACK = "127.0.0.1";

// freeLoopbackPort pede ao sistema uma porta livre no loopback, para o servidor que só
// o encaminhador alcança.
export function freeLoopbackPort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const probe = createServer().listen(0, LOOPBACK, () => {
      const address = probe.address();
      probe.close(() =>
        typeof address === "object" && address
          ? resolve(address.port)
          : reject(
              new Error(`endereço inesperado do loopback: ${String(address)}`),
            ),
      );
    });
  });
}

// Forwarder é o encaminhador de pé; close fecha a porta e derruba as conexões abertas,
// que o keep-alive dos clientes seguraria.
export type Forwarder = { close: () => Promise<void> };

// forwardDualStack escuta em :: na porta pública, atendendo IPv6 e IPv4, e repassa
// cada conexão TCP à porta do loopback. O fastmcp usa o mesmo host para escutar e para
// montar URLs, e com "::" as URLs quebram; com 0.0.0.0, o pod de um cluster IPv6 não
// recebe conexão.
export function forwardDualStack(
  port: number,
  targetPort: number,
): Promise<Forwarder> {
  const open = new Set<Socket>();
  const server = createServer((client) => {
    const target = connect(targetPort, LOOPBACK);
    open.add(client).add(target);
    client.pipe(target).pipe(client);
    client.on("close", () => open.delete(client) && target.destroy());
    target.on("close", () => open.delete(target) && client.destroy());
    client.on("error", () => target.destroy());
    target.on("error", () => client.destroy());
  });
  const close = () =>
    new Promise<void>((resolve) => {
      server.close(() => resolve());
      open.forEach((socket) => socket.destroy());
    });
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, "::", () => resolve({ close }));
  });
}
