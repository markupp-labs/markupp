import type { Clock } from "../auth/token-exchange.js";
import type { LocalFiles } from "../stdio/local-files.js";

type FakeFile = { content: string; modifiedAtMs: number };

// FakeLocalFiles troca o disco nos testes do stdio: guarda os arquivos em memória com
// o instante da última escrita pelo relógio dado, e conta as gravações privadas.
export class FakeLocalFiles implements LocalFiles {
  readonly privateWrites: string[] = [];
  private readonly files = new Map<string, FakeFile>();

  constructor(private readonly clock: Clock) {}

  // put grava como outro processo gravaria, sem passar pela escrita privada.
  put(path: string, content: string): this {
    this.files.set(path, { content, modifiedAtMs: this.clock() });
    return this;
  }

  has(path: string): boolean {
    return this.files.has(path);
  }

  async readText(path: string): Promise<string | undefined> {
    return this.files.get(path)?.content;
  }

  async writePrivate(path: string, content: string): Promise<void> {
    this.privateWrites.push(path);
    this.put(path, content);
  }

  async remove(path: string): Promise<void> {
    this.files.delete(path);
  }

  async createExclusive(path: string): Promise<boolean> {
    if (this.files.has(path)) return false;
    this.put(path, "");
    return true;
  }

  async modifiedAtMs(path: string): Promise<number | undefined> {
    return this.files.get(path)?.modifiedAtMs;
  }
}
