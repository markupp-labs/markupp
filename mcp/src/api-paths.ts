import type { SearchPage } from "./markupp-api-types.js";

// Caminhos e query strings das rotas /v1 que o HttpMarkuppApi chama.

export function notePath(id: string): string {
  return `/v1/notes/${encodeURIComponent(id)}`;
}

export function memberPath(userId: string): string {
  return `/v1/users/${encodeURIComponent(userId)}`;
}

export function vaultPath(vault: string): string {
  return `/v1/vaults/${encodeURIComponent(vault)}`;
}

export function linkRefusalsPath(vault: string): string {
  return `${vaultPath(vault)}/link-refusals`;
}

// withQuery só põe o ? quando há parâmetro.
export function withQuery(path: string, params: URLSearchParams): string {
  const query = params.toString();
  return query ? `${path}?${query}` : path;
}

// withPage põe na query string o limit e o offset que vieram, para o servidor usar o
// padrão dele nos que faltam.
export function withPage(
  params: URLSearchParams,
  page: SearchPage,
): URLSearchParams {
  if (page.limit !== undefined) params.set("limit", String(page.limit));
  if (page.offset !== undefined) params.set("offset", String(page.offset));
  return params;
}

export function myInvitePath(inviteId: string): string {
  return `/v1/me/invites/${encodeURIComponent(inviteId)}`;
}
