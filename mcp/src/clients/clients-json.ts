import { writeFileSync } from "node:fs";
import {
  AGENT_CLIENT_TEMPLATES,
  agentClientsFor,
  type AgentClient,
  type AgentClientTemplate,
} from "./catalog.js";
import { PLACEHOLDERS } from "./template-rendering.js";

// EXAMPLE_INSTANCE_URL monta o exemplo do clients.json: quem lê os modelos em outra
// linguagem (o painel em Ruby) confere a própria montagem contra ele.
export const EXAMPLE_INSTANCE_URL = "https://markupp.example.com/";

// ClientsJson é o catálogo exportado: modelos com marcadores e um exemplo montado.
export type ClientsJson = {
  placeholders: typeof PLACEHOLDERS;
  clients: AgentClientTemplate[];
  example: { instanceUrl: string; clients: AgentClient[] };
};

export function buildClientsJson(): ClientsJson {
  return {
    placeholders: PLACEHOLDERS,
    clients: AGENT_CLIENT_TEMPLATES,
    example: {
      instanceUrl: EXAMPLE_INSTANCE_URL,
      clients: agentClientsFor(EXAMPLE_INSTANCE_URL),
    },
  };
}

export function serializeClientsJson(): string {
  return `${JSON.stringify(buildClientsJson(), null, 2)}\n`;
}

// writeClientsJson grava o catálogo exportado em path.
export function writeClientsJson(path: string): void {
  writeFileSync(path, serializeClientsJson());
}
