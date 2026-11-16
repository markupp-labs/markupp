import type { ApiTokenSource } from "../session.js";

// FakeApiTokens entrega os JWTs na ordem dada e repete o último quando acabam.
export class FakeApiTokens {
  private readonly tokens: string[];

  constructor(...tokens: [string, ...string[]]) {
    this.tokens = tokens;
  }

  readonly source: ApiTokenSource = async () =>
    this.tokens.length > 1
      ? (this.tokens.shift() ?? "")
      : (this.tokens[0] ?? "");
}
