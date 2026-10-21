import type { Clock } from "../auth/token-exchange.js";
import type { LocalFiles } from "./local-files.js";

// LockDeps é o I/O da trava entre processos: o disco, o relógio e a espera.
export type LockDeps = {
  files: LocalFiles;
  clock: Clock;
  sleep: (ms: number) => Promise<void>;
};

// STALE_LOCK_MS é a idade a partir da qual o lock é dado como abandonado por um
// processo que morreu segurando-o; a seção crítica faz no máximo duas requisições com
// prazo de 5s cada.
export const STALE_LOCK_MS = 30_000;
export const LOCK_RETRY_MS = 100;

// withFileLock roda critical com o lock exclusivo em lockPath, criado com a flag wx, e
// solta o lock no fim, mesmo na falha.
export async function withFileLock<T>(
  lockPath: string,
  deps: LockDeps,
  critical: () => Promise<T>,
): Promise<T> {
  await acquireLock(lockPath, deps);
  try {
    return await critical();
  } finally {
    await deps.files.remove(lockPath);
  }
}

async function acquireLock(lockPath: string, deps: LockDeps): Promise<void> {
  while (!(await deps.files.createExclusive(lockPath))) {
    await removeIfAbandoned(lockPath, deps);
    await deps.sleep(LOCK_RETRY_MS);
  }
}

// ponytail: dois processos que acham o mesmo lock abandonado podem ambos apagá-lo, e o
// segundo apagaria o lock que o primeiro acabou de criar. Só acontece depois de um
// processo morrer segurando o lock; troque por flock (dependência nativa) se aparecer.
async function removeIfAbandoned(
  lockPath: string,
  deps: LockDeps,
): Promise<void> {
  const modifiedAt = await deps.files.modifiedAtMs(lockPath);
  if (modifiedAt === undefined) return;
  if (deps.clock() - modifiedAt <= STALE_LOCK_MS) return;
  await deps.files.remove(lockPath);
}
