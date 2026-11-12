import { MarkuppApiError, type SearchPage } from "../markupp-api.js";
import type {
  PublicMarkuppApi,
  PublicNote,
  PublicPage,
  PublicSearchHit,
  PublicTreeItem,
} from "../public-api.js";

// FakePublicMarkuppApi guarda notas públicas em memória, cada uma numa publicação de
// pasta, e responde como a leitura pública da API: not_found para id desconhecido, e
// árvore e busca só na publicação da nota lida.
export class FakePublicMarkuppApi implements PublicMarkuppApi {
  readonly calls: string[] = [];
  private readonly notes: PublicNote[] = [];
  private readonly paths = new Map<string, string>();
  private clockTick = 0;

  // withNote publica a nota na pasta folder, com o path relativo a ela.
  withNote(folder: string, path: string, content: string): PublicNote {
    this.clockTick++;
    const publicId = `pub${String(this.clockTick).padStart(9, "0")}`;
    const title = path.replace(/^.*\//, "").replace(/\.md$/, "");
    const note: PublicNote = {
      public_id: publicId,
      title,
      title_slug: title.toLowerCase().replace(/[^a-z0-9]+/g, "-"),
      content,
      updated_at: new Date(
        Date.UTC(2026, 0, 1, 0, 0, this.clockTick),
      ).toISOString(),
      indexable: false,
      publication: { kind: "folder", title: folder, root_public_id: publicId },
      links: [],
    };
    this.notes.push(note);
    this.paths.set(publicId, path);
    return note;
  }

  async getNote(publicId: string): Promise<PublicNote> {
    this.calls.push(`get ${publicId}`);
    return this.noteOf(publicId);
  }

  async listTree(
    publicId: string,
    page: SearchPage = {},
  ): Promise<PublicPage<PublicTreeItem>> {
    this.calls.push(`tree ${publicId} ${JSON.stringify(page)}`);
    const items = this.sameFolder(publicId).map((note) => ({
      public_id: note.public_id,
      title: note.title,
      title_slug: note.title_slug,
      path: this.paths.get(note.public_id) ?? "",
    }));
    return paginate(items, page);
  }

  // searchNotes casa a consulta no conteúdo, sem diferenciar caixa, e devolve o
  // trecho como o conteúdo inteiro.
  async searchNotes(
    publicId: string,
    query: string,
    page: SearchPage = {},
  ): Promise<PublicPage<PublicSearchHit>> {
    this.calls.push(`search ${publicId} ${query}`);
    const items = this.sameFolder(publicId)
      .filter((note) =>
        note.content.toLowerCase().includes(query.toLowerCase()),
      )
      .map(({ public_id, title, title_slug, content }) => ({
        public_id,
        title,
        title_slug,
        snippet: content,
        rank: 1,
      }));
    return paginate(items, page);
  }

  private sameFolder(publicId: string): PublicNote[] {
    const folder = this.noteOf(publicId).publication.title;
    return this.notes.filter((note) => note.publication.title === folder);
  }

  private noteOf(publicId: string): PublicNote {
    const found = this.notes.find((note) => note.public_id === publicId);
    if (!found)
      throw new MarkuppApiError(
        404,
        "not_found",
        "nota publica nao encontrada",
      );
    return found;
  }
}

function paginate<T>(items: T[], page: SearchPage): PublicPage<T> {
  const offset = page.offset ?? 0;
  const slice = items.slice(offset, offset + (page.limit ?? 10));
  return { items: slice, total: items.length };
}
