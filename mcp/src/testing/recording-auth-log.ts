import type { AuthFailureLog } from "../auth/http-authenticate.js";

// RecordingAuthLog guarda nos testes cada entrada que o authenticate mandaria ao log.
export class RecordingAuthLog {
  readonly entries: AuthFailureLog[] = [];

  readonly write = (entry: AuthFailureLog): void => {
    this.entries.push(entry);
  };
}
