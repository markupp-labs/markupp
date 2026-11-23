import { MarkuppApiError, type Note, type Vault } from "../markupp-api.js";
import type {
  Publication,
  PublicationList,
  PublicationTarget,
  PublishRequest,
} from "../publication.js";
import { FakeAccountEmail, FakeAccountId } from "./fake-vaults.js";

// PublishScope é o que a FakeMarkuppApi já conferiu antes de publicar: o tenant, o
// papel da conta nele, o cofre alcançado e as notas dele.
export type PublishScope = {
  tenant: string;
  role: string;
  vault: Vault;
  notes: Note[];
};

type StoredPublication = { tenant: string; publication: Publication };

// FakePublications guarda as publicações e responde como a API: publish_forbidden
// pelo papel, note_not_found e folder_not_found pelo alvo, confirmation_mismatch pelo
// nome, already_published pelo alvo repetido e publication_not_found.
export class FakePublications {
  private readonly stored: StoredPublication[] = [];
  private count = 0;

  constructor(private readonly clock: () => string) {}

  publish(scope: PublishScope, request: PublishRequest): Publication {
    requirePublisher(scope.role, request.target.kind);
    const covered = coveredNotes(scope.notes, request.target);
    const name = targetName(scope.vault, covered, request.target);
    if (request.confirm !== name)
      throw confirmationMismatch(request.confirm, name);
    this.requireUnpublished(scope, request.target);
    const publication = this.create(scope, request, name, covered.length);
    this.stored.push({ tenant: scope.tenant, publication });
    return publication;
  }

  list(tenant: string, vault?: string): PublicationList {
    const items = this.stored
      .filter((s) => s.tenant === tenant)
      .map((s) => s.publication)
      .filter((p) => vault === undefined || p.vault_id === vault);
    return { items, total: items.length };
  }

  remove(tenant: string, id: string): void {
    const index = this.stored.findIndex(
      (s) => s.tenant === tenant && s.publication.id === id,
    );
    if (index < 0)
      throw new MarkuppApiError(
        404,
        "publication_not_found",
        `publication not found: "${id}"`,
      );
    this.stored.splice(index, 1);
  }

  private requireUnpublished(
    scope: PublishScope,
    target: PublicationTarget,
  ): void {
    const existing = this.list(scope.tenant, scope.vault.id).items.find(
      (p) => targetKey(p) === keyOf(target),
    );
    if (existing)
      throw new MarkuppApiError(
        409,
        "already_published",
        `target already published as "${existing.id}"`,
      );
  }

  private create(
    scope: PublishScope,
    request: PublishRequest,
    name: string,
    noteCount: number,
  ): Publication {
    this.count += 1;
    const { target } = request;
    return {
      id: `publication-${this.count}`,
      vault_id: scope.vault.id,
      vault_name: scope.vault.name,
      kind: target.kind,
      note_id: target.kind === "note" ? target.note_id : null,
      folder_path: target.kind === "folder" ? target.path : null,
      target_name: name,
      note_count: noteCount,
      indexable: request.indexable,
      published_by: { id: FakeAccountId, name: FakeAccountEmail },
      published_at: this.clock(),
      ...(target.kind === "note" && this.notePublicId(name)),
    };
  }

  private notePublicId(name: string): {
    public_id: string;
    title_slug: string;
  } {
    const publicId = `pub${String(this.count).padStart(9, "0")}`;
    return { public_id: publicId, title_slug: name.toLowerCase() };
  }
}

// requirePublisher segue o RLS: nota por quem edita, pasta e cofre por owner e admin.
function requirePublisher(role: string, kind: PublicationTarget["kind"]): void {
  const allowed =
    kind === "note" ? role !== "viewer" : role === "owner" || role === "admin";
  if (!allowed)
    throw new MarkuppApiError(
      403,
      "publish_forbidden",
      `role "${role}" cannot publish a ${kind}: a note needs a role that edits it, a folder or vault needs owner or admin`,
    );
}

function coveredNotes(notes: Note[], target: PublicationTarget): Note[] {
  if (target.kind === "vault") return notes;
  if (target.kind === "folder")
    return required(
      notes.filter((n) => n.path.startsWith(`${target.path}/`)),
      "folder_not_found",
      target.path,
    );
  const note = notes.filter((n) => n.id === target.note_id);
  return required(note, "note_not_found", target.note_id);
}

function required(notes: Note[], code: string, value: string): Note[] {
  if (notes.length === 0)
    throw new MarkuppApiError(404, code, `nothing to publish at "${value}"`);
  return notes;
}

// targetName é o nome que a confirmação precisa repetir: o título da nota, tirado do
// nome do arquivo, o caminho da pasta ou o nome do cofre.
function targetName(
  vault: Vault,
  covered: Note[],
  target: PublicationTarget,
): string {
  if (target.kind === "vault") return vault.name;
  if (target.kind === "folder") return target.path;
  return (covered[0]?.path ?? "").replace(/^.*\//, "").replace(/\.md$/, "");
}

function confirmationMismatch(got: string, expected: string): MarkuppApiError {
  return new MarkuppApiError(
    422,
    "confirmation_mismatch",
    `confirm ${JSON.stringify(got)} does not match the target name ${JSON.stringify(expected)}`,
  );
}

function keyOf(target: PublicationTarget): string {
  if (target.kind === "note") return `note:${target.note_id}`;
  if (target.kind === "folder") return `folder:${target.path}`;
  return "vault";
}

function targetKey(publication: Publication): string {
  if (publication.kind === "note") return `note:${publication.note_id}`;
  if (publication.kind === "folder") return `folder:${publication.folder_path}`;
  return "vault";
}
