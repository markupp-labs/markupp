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
// servidor cria quando a nota cita o título de outra no texto, related é a por texto
// parecido e semantic a por sentido, entre notas que tratam do mesmo assunto; as três
// são automáticas (ADR-0053).
export type LinkType =
  | "explicit"
  | "embed"
  | "typed"
  | "inferred"
  | "mentioned"
  | "related"
  | "semantic";

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

// EmbeddingProvider é o provedor de embedding da instância (ADR-0057).
export type EmbeddingProvider = "none" | "openai-compat" | "local";

// InstanceEmbedding é a escolha como GET /v1/instance/embedding devolve: nunca a chave,
// só se há uma; locked diz que a configuração de quem hospeda vence.
export type InstanceEmbedding = {
  provider: EmbeddingProvider;
  url: string;
  model: string;
  api_key_set: boolean;
  locked: boolean;
  host_provider: string;
  model_status: {
    state: "downloading" | "ready" | "error";
    error: string;
    changed_at: string;
  } | null;
};

// InstanceEmbeddingUpdate é o corpo do PUT. Sem api_key a chave guardada fica; vazia a apaga.
export type InstanceEmbeddingUpdate = {
  provider: EmbeddingProvider;
  url?: string;
  model?: string;
  api_key?: string;
};

// LoginProvider é um provedor de login externo (OIDC) da instância como a API devolve:
// nunca o client secret, só se há um; locked diz que quem hospeda trava o issuer.
export type LoginProvider = {
  id: string;
  name: string;
  issuer: string;
  client_id: string;
  client_secret_set: boolean;
  enabled: boolean;
  locked: boolean;
  callback_url: string;
  linked_accounts: number;
};

// LoginProviderList é GET /v1/instance/login-providers; locked diz que quem hospeda
// trava a lista inteira.
export type LoginProviderList = {
  locked: boolean;
  providers: LoginProvider[];
};

// LoginProviderChange é o corpo do PUT: substitui todos os campos não secretos. Sem
// client_secret o segredo guardado fica.
export type LoginProviderChange = {
  name: string;
  issuer: string;
  client_id: string;
  client_secret?: string;
  enabled: boolean;
};

// LoginProviderCreation é o corpo do POST, com o id que nunca muda depois.
export type LoginProviderCreation = LoginProviderChange & {
  id: string;
  client_secret: string;
};

// LoginProviderRemoval é a resposta do DELETE: quantas contas perderam a ligação.
export type LoginProviderRemoval = { removed_links: number };

// Session é uma sessão de login da conta (ADR-0061); current marca a de quem pede.
export type Session = {
  id: string;
  ip: string;
  user_agent: string;
  created_at: string;
  last_used_at: string;
  current: boolean;
};

// AccountStatus filtra as contas da instância.
export type AccountStatus = "active" | "blocked";

// AccountQuery filtra GET /v1/instance/accounts; q é parte do email.
export type AccountQuery = {
  q?: string;
  status?: AccountStatus;
  limit?: number;
  offset?: number;
};

// InstanceAccount é a conta como o administrador da instância a vê.
export type InstanceAccount = {
  id: string;
  email: string;
  email_verified: boolean;
  login_methods: string[];
  created_at: string;
  last_access_at: string | null;
  blocked: boolean;
};

// InstanceAccountDetail traz também os tenants que somem com a conta.
export type InstanceAccountDetail = InstanceAccount & {
  sole_owner_tenants: { id: string; name: string; notes: number }[];
};

// AppAuthorization é um app autorizado de uma conta, com a origem do último uso.
export type AppAuthorization = {
  id: string;
  client_id: string;
  client_name: string;
  scopes: string[];
  created_at: string;
  updated_at: string;
  ip: string;
  user_agent: string;
  last_used_at?: string;
};

// IpBlock é um IP ou faixa bloqueado na instância.
export type IpBlock = {
  id: string;
  cidr: string;
  note: string;
  created_at: string;
};

export type IpBlockCreation = { cidr: string; note?: string };

// AuditResult é o resultado de um evento do histórico.
export type AuditResult = "success" | "failure" | "blocked";

// AuditQuery filtra o histórico; since e until são RFC 3339, até until exclusive.
export type AuditQuery = {
  account?: string;
  action?: string;
  result?: AuditResult;
  ip?: string;
  since?: string;
  until?: string;
  limit?: number;
  offset?: number;
};

// AuditEvent é um evento do histórico de acessos e mudanças sensíveis.
export type AuditEvent = {
  id: string;
  actor_id: string;
  actor_email: string;
  action: string;
  target: string;
  result: AuditResult;
  ip: string;
  user_agent: string;
  occurred_at: string;
};

// Retention é o prazo de guarda do histórico e do IP; host_locked diz que quem hospeda o fixou.
export type Retention = { days: number; host_locked: boolean };
