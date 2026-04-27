/**
 * Disk-backed named session registry.
 *
 * Each named session lives at `<sessionsDir>/<name>/`:
 *   initialized   – marker file written on first turn (ISO timestamp)
 *   meta.json     – session metadata (repo, scope, turn count)
 *   report-<ts>.json – saved evidence reports, one per completed turn
 *
 * acpx owns its own session persistence layer. This module owns only the
 * ghx-sidecar-specific artifacts that live alongside it.
 */

import { existsSync, mkdirSync, readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import type { SidecarReport } from '../schema.js';

/** Lightweight metadata file written into each session directory. */
export interface SessionMeta {
  /** Bare session name (without the `ghx-sidecar:` prefix). */
  name: string;
  /** GitHub repo under investigation (`owner/repo`). */
  repo: string;
  /** Short label describing the investigation scope. */
  scope: string;
  /** Total number of turns sent so far. */
  turnCount: number;
  /** ISO timestamp of first turn. */
  createdAt: string;
  /** ISO timestamp of most recent turn. */
  updatedAt: string;
}

/** Check whether a named session has been initialized (i.e. sidecar prompt sent). */
export function isInitialized(sessionsDir: string, name: string): boolean {
  return existsSync(join(sessionsDir, name, 'initialized'));
}

/**
 * Mark a session as initialized and write its initial metadata.
 * Creates the session directory if needed.
 */
export function initSession(sessionsDir: string, name: string, repo: string, scope: string): void {
  const dir = join(sessionsDir, name);
  mkdirSync(dir, { recursive: true });

  const now = new Date().toISOString();
  writeFileSync(join(dir, 'initialized'), now, 'utf8');

  const meta: SessionMeta = { name, repo, scope, turnCount: 0, createdAt: now, updatedAt: now };
  writeFileSync(join(dir, 'meta.json'), JSON.stringify(meta, null, 2), 'utf8');
}

/** Read a session's metadata. Returns undefined if the session does not exist. */
export function readMeta(sessionsDir: string, name: string): SessionMeta | undefined {
  const path = join(sessionsDir, name, 'meta.json');
  if (!existsSync(path)) return undefined;
  return JSON.parse(readFileSync(path, 'utf8')) as SessionMeta;
}

/** Increment the turn counter and update the `updatedAt` timestamp. */
export function recordTurn(sessionsDir: string, name: string): void {
  const meta = readMeta(sessionsDir, name);
  if (meta === undefined) return;
  meta.turnCount += 1;
  meta.updatedAt = new Date().toISOString();
  writeFileSync(join(sessionsDir, name, 'meta.json'), JSON.stringify(meta, null, 2), 'utf8');
}

/** Persist a structured evidence report for a session turn. */
export function saveReport(sessionsDir: string, name: string, report: SidecarReport): string {
  const dir = join(sessionsDir, name);
  mkdirSync(dir, { recursive: true });
  const ts = Date.now();
  const path = join(dir, `report-${ts}.json`);
  writeFileSync(path, JSON.stringify(report, null, 2), 'utf8');
  return path;
}

/** List all session names that exist on disk. */
export function listSessions(sessionsDir: string): SessionMeta[] {
  if (!existsSync(sessionsDir)) return [];
  return readdirSync(sessionsDir, { withFileTypes: true })
    .filter(e => e.isDirectory())
    .flatMap(e => {
      const meta = readMeta(sessionsDir, e.name);
      return meta !== undefined ? [meta] : [];
    })
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
}

/** List saved report paths for a session, newest first. */
export function listReports(sessionsDir: string, name: string): string[] {
  const dir = join(sessionsDir, name);
  if (!existsSync(dir)) return [];
  return readdirSync(dir)
    .filter(f => f.startsWith('report-') && f.endsWith('.json'))
    .sort()
    .reverse()
    .map(f => join(dir, f));
}
