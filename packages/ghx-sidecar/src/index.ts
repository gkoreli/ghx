#!/usr/bin/env node

import { createSidecarPrompt } from "./prompt.js";
import type { Backend, SidecarRequest } from "./schema.js";

export { createSidecarPrompt } from "./prompt.js";
export type {
  Backend,
  Claim,
  Evidence,
  RelevantFile,
  SidecarReport,
  SidecarRequest,
  SidecarRuntime,
} from "./schema.js";

function parseArgs(argv: string[]): SidecarRequest {
  let session = "";
  let repo = "";
  let depth: NonNullable<SidecarRequest["depth"]> = "normal";
  let output: NonNullable<SidecarRequest["output"]> = "evidence";
  const allowedBackends: Backend[] = [];
  const questionParts: string[] = [];

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === "--session") {
      session = requireValue(argv, ++i, "--session");
    } else if (arg === "--repo") {
      repo = requireValue(argv, ++i, "--repo");
    } else if (arg === "--depth") {
      depth = requireValue(argv, ++i, "--depth") as NonNullable<SidecarRequest["depth"]>;
    } else if (arg === "--backend") {
      allowedBackends.push(requireValue(argv, ++i, "--backend") as Backend);
    } else if (arg === "--output") {
      output = requireValue(argv, ++i, "--output") as NonNullable<SidecarRequest["output"]>;
    } else {
      questionParts.push(arg ?? "");
    }
  }

  if (!session) throw new Error("--session is required");
  if (!repo) throw new Error("--repo is required");
  const question = questionParts.join(" ").trim();
  if (!question) throw new Error("question is required");

  return {
    session,
    repo,
    question,
    depth,
    output,
    allowedBackends: allowedBackends.length > 0 ? allowedBackends : ["remote"],
  };
}

function requireValue(argv: string[], index: number, flag: string): string {
  const value = argv[index];
  if (!value || value.startsWith("--")) {
    throw new Error(`${flag} requires a value`);
  }
  return value;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  try {
    const request = parseArgs(process.argv.slice(2));
    process.stdout.write(createSidecarPrompt(request));
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    process.stderr.write(`ghx-sidecar: ${message}\n`);
    process.exit(1);
  }
}
