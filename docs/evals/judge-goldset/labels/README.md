# Gold-Set Labels

Human gold labels for judge calibration (ADR-0023.1 D5). Produced strictly
under `../PROTOCOL.md` (`goldset-protocol-v1`); one JSON per episode, named
`<labelId>.json` with label IDs from `../CANDIDATES.md`.

Status: **no gold labels exist yet.** The founder labeling session has not
run. The only file here besides this README and the packet used by it is:

- `EXAMPLE-gs-009.json` — a worked example produced while writing the
  protocol, labeled from the committed packet `bundles/gs-009.bundle.json`.
  It is marked **EXAMPLE-NOT-GOLD**: it was not produced in a blind founder
  session and is **excluded from κ computation**, as is any file with the
  `EXAMPLE-` prefix.

Rules recap (see PROTOCOL.md for the full contract):

- Labels are written blind from `bundles/gs-NNN.bundle.json` packets only;
  `episodeId` is filled in after the session (unblinding), and scores never
  change after unblinding.
- Commit each label together with the packet it was labeled from, so κ stays
  recomputable from committed artifacts.
- Skipped episodes (recognized from prior debugging, or anything that looks
  like an escape-hatch answer) are recorded here:

| label id | skipped because |
| --- | --- |
| (none yet) | |
