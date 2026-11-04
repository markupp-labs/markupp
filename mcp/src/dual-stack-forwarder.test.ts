import { connect, createServer, type Server } from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import {
  forwardDualStack,
  freeLoopbackPort,
  type Forwarder,
} from "./dual-stack-forwarder.js";

// EchoServer devolve no loopback o que recebe, no papel do fastmcp.
class EchoServer {
  private server: Server | undefined;
  port = 0;

  async start(): Promise<this> {
    this.port = await freeLoopbackPort();
    this.server = createServer((socket) => socket.pipe(socket));
    await new Promise<void>((resolve) =>
      this.server?.listen(this.port, "127.0.0.1", resolve),
    );
    return this;
  }

  stop(): Promise<void> {
    return new Promise((resolve) => this.server?.close(() => resolve()));
  }
}

// echoThrough conecta no host pela porta, manda o texto e espera ele voltar.
function echoThrough(
  host: string,
  port: number,
  text: string,
): Promise<string> {
  return new Promise((resolve, reject) => {
    const socket = connect(port, host, () => socket.write(text));
    socket.on("data", (data) => {
      resolve(data.toString());
      socket.destroy();
    });
    socket.on("error", reject);
  });
}

let echo: EchoServer;
let forwarder: Forwarder;

afterEach(async () => {
  await forwarder.close();
  await echo.stop();
});

describe("encaminhador de pilha dupla", () => {
  it("repassa conexões por IPv6 e por IPv4 ao loopback", async () => {
    echo = await new EchoServer().start();
    const port = await freeLoopbackPort();
    forwarder = await forwardDualStack(port, echo.port);

    expect(await echoThrough("::1", port, "seis")).toBe("seis");
    expect(await echoThrough("127.0.0.1", port, "quatro")).toBe("quatro");
  });

  it("fecha mesmo com uma conexão aberta", async () => {
    echo = await new EchoServer().start();
    const port = await freeLoopbackPort();
    forwarder = await forwardDualStack(port, echo.port);
    const idle = connect(port, "::1");
    await new Promise((resolve) => idle.once("connect", resolve));

    await forwarder.close();

    await new Promise((resolve) => idle.once("close", resolve));
    forwarder = { close: async () => {} };
  });
});
