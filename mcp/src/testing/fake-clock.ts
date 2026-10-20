// FakeClock troca o relógio nos testes: o tempo fica parado até o teste mandar andar.
export class FakeClock {
  private nowMs: number;

  constructor(startMs: number = Date.UTC(2026, 9, 20, 12)) {
    this.nowMs = startMs;
  }

  advance(ms: number): void {
    this.nowMs += ms;
  }

  readonly now = (): number => this.nowMs;
}
