import {
  MarkuppApiError,
  type Link,
  type LinkedNote,
  type LinkQuery,
  type LinkRefusal,
  type LinkType,
  type Note,
  type SearchPage,
} from "../markupp-api.js";

type StoredRefusal = LinkRefusal & { vault: string };

// FakeLinks guarda as ligações e as recusas de ligação de cada cofre. A recusa esconde a
// ligação automática (mentioned ou related) entre o par, nos dois sentidos, como a
// API. Tenant, papel e cofre ficam com a FakeMarkuppApi, que confere tudo antes de
// chamar.
export class FakeLinks {
  private readonly links: Link[] = [];
  private readonly refusals: StoredRefusal[] = [];
  private count = 0;

  constructor(private readonly clock: () => string) {}

  add(source: Note, target: Note, type: LinkType): void {
    const to = linkedNote(target);
    this.links.push({
      source: linkedNote(source),
      target: to,
      cited_title: to.title,
      type,
      predicate: null,
      inferred_target: null,
    });
  }

  list(note: string, query: LinkQuery): Link[] {
    const outgoing = query.direction === "outgoing";
    return paged(
      this.links
        .filter((l) => (outgoing ? l.source.id : l.target?.id) === note)
        .filter((l) => query.type === undefined || l.type === query.type)
        .filter((l) => !this.hidden(l)),
      query,
    );
  }

  // refuse recusa o par das notas do cofre; recusar de novo devolve a recusa que existe.
  refuse(vault: string, noteIds: string[], notes: Note[]): LinkRefusal {
    const pair = notePair(vault, noteIds, notes);
    const existing = this.refusals.find(
      (r) => r.vault === vault && samePair(r, noteIds),
    );
    if (existing) return publicRefusal(existing);
    this.count += 1;
    const refusal = {
      id: `refusal-${this.count}`,
      notes: pair,
      created_at: this.clock(),
    };
    this.refusals.unshift({ ...refusal, vault });
    return refusal;
  }

  refusalsIn(vault: string, page: SearchPage): LinkRefusal[] {
    const inVault = this.refusals.filter((r) => r.vault === vault);
    return paged(inVault, page).map(publicRefusal);
  }

  undo(vault: string, refusalId: string): void {
    const index = this.refusals.findIndex(
      (r) => r.vault === vault && r.id === refusalId,
    );
    if (index < 0)
      throw new MarkuppApiError(
        404,
        "link_refusal_not_found",
        `link refusal not found in the vault: "${refusalId}"`,
      );
    this.refusals.splice(index, 1);
  }

  private hidden(link: Link): boolean {
    const automatic =
      link.type === "mentioned" ||
      link.type === "related" ||
      link.type === "semantic";
    if (!automatic || !link.target) return false;
    const ids = [link.source.id, link.target.id];
    return this.refusals.some((r) => samePair(r, ids));
  }
}

// notePair acha as duas notas do cofre, na ordem do id, como a API devolve o par.
function notePair(
  vault: string,
  noteIds: string[],
  notes: Note[],
): LinkedNote[] {
  const [first, second] = noteIds;
  if (first === second)
    throw new MarkuppApiError(
      400,
      "same_note",
      `the pair repeats note "${first}"`,
    );
  return noteIds
    .map((id) => vaultNote(vault, id, notes))
    .sort((x, y) => x.id.localeCompare(y.id));
}

function vaultNote(vault: string, id: string, notes: Note[]): LinkedNote {
  const note = notes.find((n) => n.id === id && n.vault_id === vault);
  if (note) return linkedNote(note);
  throw new MarkuppApiError(
    422,
    "note_outside_vault",
    `note does not belong to the vault: "${id}"`,
  );
}

// samePair diz se a recusa é do par ids, em qualquer ordem.
function samePair(refusal: LinkRefusal, ids: string[]): boolean {
  return refusal.notes.every((note) => ids.includes(note.id));
}

// linkedNote resume a nota; o título é o último segmento do caminho, sem .md.
function linkedNote(note: Note): LinkedNote {
  const title = note.path.slice(note.path.lastIndexOf("/") + 1);
  return { id: note.id, title: title.replace(/\.md$/, ""), path: note.path };
}

function paged<T>(items: T[], page: SearchPage): T[] {
  const offset = page.offset ?? 0;
  return items.slice(offset, offset + (page.limit ?? 10));
}

function publicRefusal(refusal: StoredRefusal): LinkRefusal {
  const { id, notes, created_at } = refusal;
  return { id, notes, created_at };
}
