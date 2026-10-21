import { randomUUID } from "node:crypto";
import {
  chmod,
  mkdir,
  readFile,
  rename,
  rm,
  stat,
  writeFile,
} from "node:fs/promises";
import { dirname } from "node:path";

// LocalFiles é o disco da máquina da pessoa visto pela credencial do stdio: ler, gravar
// só para o dono, apagar e o lock por criação exclusiva.
export interface LocalFiles {
  // readText devolve undefined quando o arquivo não existe.
  readText(path: string): Promise<string | undefined>;
  // writePrivate grava de forma atômica, diretório 0700 e arquivo 0600.
  writePrivate(path: string, content: string): Promise<void>;
  // remove não falha quando o arquivo já não existe.
  remove(path: string): Promise<void>;
  // createExclusive cria o arquivo vazio e devolve false se ele já existe.
  createExclusive(path: string): Promise<boolean>;
  // modifiedAtMs devolve undefined quando o arquivo não existe.
  modifiedAtMs(path: string): Promise<number | undefined>;
}

const PRIVATE_DIR_MODE = 0o700;
const PRIVATE_FILE_MODE = 0o600;

// NodeLocalFiles é o LocalFiles pelo node:fs.
export class NodeLocalFiles implements LocalFiles {
  async readText(path: string): Promise<string | undefined> {
    return readFile(path, "utf8").catch(undefinedWhen("ENOENT"));
  }

  // writePrivate grava num temporário ao lado, fixa 0600 e troca pelo rename, para
  // outro processo nunca ler o arquivo pela metade.
  async writePrivate(path: string, content: string): Promise<void> {
    const dir = dirname(path);
    await mkdir(dir, { recursive: true, mode: PRIVATE_DIR_MODE });
    await chmod(dir, PRIVATE_DIR_MODE);
    const temporary = `${path}.${randomUUID()}.tmp`;
    try {
      await writeFile(temporary, content, {
        mode: PRIVATE_FILE_MODE,
        flag: "wx",
      });
      await chmod(temporary, PRIVATE_FILE_MODE);
      await rename(temporary, path);
    } finally {
      await rm(temporary, { force: true });
    }
  }

  async remove(path: string): Promise<void> {
    await rm(path, { force: true });
  }

  async createExclusive(path: string): Promise<boolean> {
    const created = writeFile(path, "", {
      mode: PRIVATE_FILE_MODE,
      flag: "wx",
    });
    return created.then(() => true, falseWhen("EEXIST"));
  }

  async modifiedAtMs(path: string): Promise<number | undefined> {
    const stats = await stat(path).catch(undefinedWhen("ENOENT"));
    return stats?.mtimeMs;
  }
}

function hasCode(error: unknown, code: string): boolean {
  return error instanceof Error && "code" in error && error.code === code;
}

function undefinedWhen(code: string): (error: unknown) => undefined {
  return (error) => {
    if (hasCode(error, code)) return undefined;
    throw error;
  };
}

function falseWhen(code: string): (error: unknown) => false {
  return (error) => {
    if (hasCode(error, code)) return false;
    throw error;
  };
}
