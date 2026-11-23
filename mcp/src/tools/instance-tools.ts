import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  readOnlyAnnotations,
  writeAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

const embeddingChoice = z.object({
  provider: z
    .enum(["none", "openai-compat", "local"])
    .describe(
      "none turns semantic links off; local runs the built-in model on the server, downloading it first; openai-compat calls an OpenAI-compatible /embeddings endpoint.",
    ),
  url: z
    .string()
    .max(2048)
    .optional()
    .describe(
      "Base URL of the OpenAI-compatible endpoint, without /embeddings, such as https://api.openai.com/v1. Required with openai-compat.",
    ),
  model: z
    .string()
    .max(2048)
    .optional()
    .describe("Model name the endpoint expects. Required with openai-compat."),
  api_key: z
    .string()
    .max(2048)
    .optional()
    .describe(
      "Endpoint API key, stored encrypted and never returned. Omit to keep the stored key; an empty string removes it. Switching to another openai-compat URL requires sending it again; switching to local or none without it removes the stored key.",
    ),
});

// addInstanceTools registra get_embedding_provider e set_embedding_provider, do
// administrador da instância (ADR-0057). A API recusa as outras contas.
export function addInstanceTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "get_embedding_provider",
    description:
      "Instance admins only. Returns the server's embedding provider for semantic links, whether an API key is set (never the key), whether the hosting configuration locks the choice, and the model status (downloading, ready, error).",
    parameters: z.object({}),
    annotations: readOnlyAnnotations,
    execute: async (_args, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.getInstanceEmbedding()));
    },
  });
  addSetEmbeddingProvider(server, factory);
}

function addSetEmbeddingProvider(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "set_embedding_provider",
    description:
      "Instance admins only, when the user asked to change it. Sets the server's embedding provider; the server switches within a minute and recomputes every note's vector when the model changes. Saving the same choice again retries a failed download. Refused while the hosting configuration locks the choice.",
    parameters: embeddingChoice,
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async (choice, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(
        await callApi(() => api.setInstanceEmbedding(choice)),
      );
    },
  });
}
