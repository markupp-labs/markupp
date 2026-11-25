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

// addSessionTools registra as ferramentas das sessões de login da própria conta (ADR-0061).
// A API só as aceita com o escopo administracao.
export function addSessionTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_my_sessions",
    description:
      "Lists the active sign-in sessions of your account with IP, user agent, creation and last use. current marks the session that made the call; through an agent none is current, since agents use authorized apps.",
    parameters: z.object({}),
    annotations: readOnlyAnnotations,
    execute: async (_args, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listMySessions()));
    },
  });
  server.addTool({
    name: "end_my_session",
    description:
      "Only when the user asked for it, such as for a device they do not recognize. Ends a sign-in session of your account; that device must sign in again within minutes.",
    parameters: z.object({
      id: z.string().min(1).describe("Session id from list_my_sessions."),
    }),
    annotations: {
      ...writeAnnotations,
      destructiveHint: true,
      idempotentHint: true,
    },
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      await callApi(() => api.endMySession(id));
      return `Session ${id} ended.`;
    },
  });
}
