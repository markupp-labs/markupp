import { MarkuppApiError, type SearchPage } from "../markupp-api.js";
import type {
  PublicListPage,
  PublicMarkuppApi,
  PublicNote,
  PublicNoteItem,
  PublicSearchHit,
} from "../public-api.js";

// FakePublicMarkuppApi guarda cofres publicados em memória e responde como a leitura
// pública da API: not_found tanto para slug sem cofre quanto para nota fora dele.
export class FakePublicMarkuppApi implements PublicMarkuppApi {
  readonly calls: string[] = [];
  private readonly vaults = new Map<string, PublicNote[]>();
  private clockTick = 0;

  withVault(slug: string): this {
    this.vaults.set(slug, []);
    return this;
  }

  // withNote grava a nota no cofre slug, que precisa existir.
  withNote(slug: string, path: string, content: string): PublicNote {
    this.clockTick++;
    const at = new Date(Date.UTC(2026, 0, 1, 0, 0, this.clockTick));
    const note = {
      id: `note-${this.clockTick}`,
      title: path.replace(/^.*\//, "").replace(/\.md$/, ""),
      path,
      content,
      created_at: at.toISOString(),
      updated_at: at.toISOString(),
    };
    this.notesOf(slug).push(note);
    return note;
  }

  async listNotes(
    slug: string,
    page: PublicListPage = {},
  ): Promise<PublicNoteItem[]> {
    this.calls.push(`list ${slug} ${JSON.stringify(page)}`);
    const sorted = [...this.notesOf(slug)].sort((a, b) =>
      a.path.localeCompare(b.path),
    );
    return paginate(sorted, page).map(({ id, title, path, updated_at }) => ({
      id,
      title,
      path,
      updated_at,
    }));
  }

  async getNote(slug: string, id: string): Promise<PublicNote> {
    this.calls.push(`get ${slug} ${id}`);
    const found = this.notesOf(slug).find((note) => note.id === id);
    if (!found)
      throw new MarkuppApiError(404, "not_found", `nota "${id}" não existe`);
    return found;
  }

  // searchNotes casa a consulta no conteúdo, sem diferenciar caixa, da mais nova para a
  // mais velha, e devolve o trecho como o conteúdo inteiro.
  async searchNotes(
    slug: string,
    query: string,
    page: SearchPage = {},
  ): Promise<PublicSearchHit[]> {
    this.calls.push(`search ${slug} ${query}`);
    const matches = this.notesOf(slug)
      .filter((note) =>
        note.content.toLowerCase().includes(query.toLowerCase()),
      )
      .reverse();
    return paginate(matches, page).map(({ id, title, path, content }) => ({
      note: { id, title, path },
      rank: 1,
      snippet: content,
    }));
  }

  private notesOf(slug: string): PublicNote[] {
    const notes = this.vaults.get(slug);
    if (!notes)
      throw new MarkuppApiError(
        404,
        "not_found",
        "cofre publico nao encontrado",
      );
    return notes;
  }
}

function paginate<T>(items: T[], page: SearchPage): T[] {
  const offset = page.offset ?? 0;
  return items.slice(offset, offset + (page.limit ?? 10));
}
