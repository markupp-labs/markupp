// env.wikilinks de um cofre em memória: caminho da nota -> url. Nome sem nota devolve null, e missingTitle
// diz o motivo, como o painel faz.
export class FakeNoteDirectory {
  constructor(urlsByPath) {
    this.urlsByPath = urlsByPath
  }

  resolve(name) {
    return this.urlsByPath[name] ?? null
  }

  missingTitle(name) {
    return `nota ${name} não existe`
  }
}
