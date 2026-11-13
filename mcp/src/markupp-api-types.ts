// Tipos das respostas e pedidos da API REST do markupp que as ferramentas MCP usam.

// TenantMembership é um vínculo da conta como GET /v1/me/tenants devolve.
export type TenantMembership = { id: string; name: string; role: string };

// NoteSummary é o que a busca devolve de cada nota, sem o conteúdo.
export type NoteSummary = {
  id: string;
  vault_id: string;
  path: string;
  updated_at: string;
};

// Note é a nota inteira, como as rotas /v1/notes/{id} e POST /v1/notes devolvem.
export type Note = NoteSummary & { content: string; created_at: string };

// FoundNote é a nota que a busca acha; a busca num cofre não traz updated_at.
export type FoundNote = Omit<NoteSummary, "updated_at"> & {
  updated_at?: string;
};

// Account é a conta do token, como GET /v1/auth/me devolve, sem os campos de login.
export type Account = { id: string; email: string };

// Vault é um cofre de que a conta é membro, como GET /v1/vaults devolve.
export type Vault = { id: string; name: string; created_at: string };

// SearchPage pagina a busca; o servidor limita limit a 1..100 e usa 10 se faltar.
export type SearchPage = { limit?: number; offset?: number };

// NoteRevision é a nova versão da nota com o updated_at lido, que o servidor compara
// para recusar com 409 a escrita feita sobre uma versão velha.
export type NoteRevision = {
  path: string;
  content: string;
  lastModifiedAt: string;
};

// Role é o papel de uma conta no tenant, do maior para o menor.
export type Role = "owner" | "admin" | "member" | "viewer";

// Member é um membro do tenant como GET /v1/users devolve, ou do cofre como GET
// /v1/vaults/{id}/members devolve, com o papel no tenant; id é o id da conta.
export type Member = {
  id: string;
  email: string;
  role: string;
  joined_at: string;
};

// TenantInvite é um convite do tenant como GET /v1/users/invites devolve; accepted_at
// só vem no convite já aceito.
export type TenantInvite = {
  id: string;
  email: string;
  role: string;
  invited_by?: string;
  expires_at: string;
  accepted_at?: string;
  created_at: string;
};

// IssuedInvite é o convite recém-emitido; token só vem quando o servidor não manda o
// convite por email e o administrador repassa o token à mão.
export type IssuedInvite = TenantInvite & { token?: string };

// PendingInvite é um convite endereçado à conta como GET /v1/me/invites devolve.
export type PendingInvite = {
  id: string;
  tenant_id: string;
  tenant_name: string;
  role: string;
  expires_at: string;
};

// LinkedNote é a nota numa ligação ou numa recusa de ligação, como a API a resume.
export type LinkedNote = { id: string; title: string; path: string };

// LinkType é o tipo da ligação; mentioned é a ligação por menção, automática, que o
// servidor cria quando a nota cita o título de outra no texto, e related é a por texto
// parecido, também automática (ADR-0053).
export type LinkType =
  "explicit" | "embed" | "typed" | "inferred" | "mentioned" | "related";

// Link é uma ligação como GET /v1/notes/{id}/links e /backlinks devolvem; target vem
// null na ligação não resolvida, e predicate só vem na ligação tipada.
export type Link = {
  source: LinkedNote;
  target: LinkedNote | null;
  cited_title: string;
  type: LinkType;
  predicate: string | null;
  inferred_target: LinkedNote | null;
};

// LinkQuery escolhe o sentido das ligações, outgoing as que a nota faz e incoming as
// que chegam nela, o tipo e a página.
export type LinkQuery = SearchPage & {
  direction: "outgoing" | "incoming";
  type?: LinkType;
};

// LinkRefusal é o par de notas do cofre em que o servidor não cria ligação automática.
export type LinkRefusal = {
  id: string;
  notes: LinkedNote[];
  created_at: string;
};
