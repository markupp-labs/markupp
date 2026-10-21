// FakeFiles troca a leitura de arquivos nos testes: cada arquivo pode aparecer só
// depois de algumas leituras, como o segredo que o Go escreve enquanto sobe.
export class FakeFiles {
  readonly reads: string[] = [];
  private readonly files = new Map<
    string,
    { content: string; failedReads: number }
  >();

  write(path: string, content: string, afterReads: number = 0): this {
    this.files.set(path, { content, failedReads: afterReads });
    return this;
  }

  readonly readFile = async (path: string): Promise<string> => {
    this.reads.push(path);
    const file = this.files.get(path);
    if (!file || file.failedReads-- > 0) {
      throw new Error(`ENOENT: arquivo ${path} inexistente`);
    }
    return file.content;
  };
}
