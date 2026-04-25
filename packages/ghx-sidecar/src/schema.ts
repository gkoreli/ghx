export type Backend = "remote" | "codemap" | "local";

export interface SidecarRequest {
  session: string;
  repo: string;
  question: string;
  depth?: "cheap" | "normal" | "deep";
  allowedBackends?: Backend[];
  output?: "evidence" | "json";
}

export interface Claim {
  summary: string;
  evidence?: string;
}

export interface RelevantFile {
  path: string;
  reason: string;
}

export interface Evidence {
  source: string;
  summary: string;
}

export interface SidecarReport {
  answer: string;
  verified: Claim[];
  inferred: Claim[];
  unverified: Claim[];
  relevantFiles: RelevantFile[];
  evidence: Evidence[];
  backendsUsed: string[];
  commandsRun: string[];
  uncertainty: string[];
  nextReads: string[];
}

export interface SidecarRuntime {
  ask(request: SidecarRequest): Promise<SidecarReport>;
}
