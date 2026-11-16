// PublicationTarget é o que se publica (ADR-0048): uma nota, uma pasta pelo caminho
// sem barra no começo nem no fim, ou o cofre inteiro.
export type PublicationTarget =
  | { kind: "note"; note_id: string }
  | { kind: "folder"; path: string }
  | { kind: "vault" };

// PublishRequest é o corpo de POST /v1/vaults/{id}/publications; confirm é o nome do
// alvo que o usuário confirmou: título da nota, caminho da pasta ou nome do cofre.
export type PublishRequest = {
  target: PublicationTarget;
  confirm: string;
  indexable: boolean;
};

// Publication é a publicação como a API devolve; public_id e title_slug só vêm na
// publicação de nota.
export type Publication = {
  id: string;
  vault_id: string;
  vault_name: string;
  kind: PublicationTarget["kind"];
  note_id: string | null;
  folder_path: string | null;
  target_name: string;
  note_count: number;
  indexable: boolean;
  published_by: { id: string; name: string };
  published_at: string;
  public_id?: string;
  title_slug?: string;
};

// PublicationList é a resposta de GET /v1/publications.
export type PublicationList = { items: Publication[]; total: number };
