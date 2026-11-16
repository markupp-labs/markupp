import { MarkuppApiError, type PendingInvite } from "../markupp-api.js";

// FakeMyInvites guarda os convites endereçados à conta da sessão; responder um convite
// o tira da lista, e o que não está nela responde invite_not_found.
export class FakeMyInvites {
  private readonly pending: PendingInvite[] = [];

  add(invite: PendingInvite): PendingInvite {
    this.pending.push(invite);
    return invite;
  }

  list(): PendingInvite[] {
    return [...this.pending];
  }

  take(inviteId: string): PendingInvite {
    const index = this.pending.findIndex((invite) => invite.id === inviteId);
    const [invite] = index < 0 ? [] : this.pending.splice(index, 1);
    if (!invite) throw inviteNotFound();
    return invite;
  }
}

export function inviteNotFound(): MarkuppApiError {
  return new MarkuppApiError(
    404,
    "invite_not_found",
    "invite not found, expired or already answered",
  );
}
