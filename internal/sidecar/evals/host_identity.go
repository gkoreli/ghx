package evals

import (
	"encoding/json"
	"fmt"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// HostTaskIdentityHashes builds the ADR-0032.1 S3 frozen-identity inventory
// for a host-task run, recorded in the run manifest
// (RunManifest.HostTaskIdentityHashes) so a later run — or a human auditor —
// can verify the measured prompt contracts and the recon tool surface were
// byte-identical:
//
//   - hostPromptContract: the full arm-A host prompt template, placeholders
//     included (per-trial workspace paths and issue text never change
//     identity).
//   - armBPromptContract: the full arm-B template — the base prompt plus the
//     recon skill/prompt contract that defines the sidecar arm.
//   - reconToolSchema: the canonical JSON of sidecar.ReconMCPTool (name,
//     description, input schema) — the exact tool definition `ghx serve
//     --recon` registers, shared source so served schema and hashed schema
//     cannot drift.
func HostTaskIdentityHashes() ([]BaselineReuseHashRecord, error) {
	schema, err := json.Marshal(sidecar.ReconMCPTool())
	if err != nil {
		return nil, fmt.Errorf("evals: marshal recon tool schema: %w", err)
	}
	return []BaselineReuseHashRecord{
		{
			Name:      "hostPromptContract",
			Algorithm: "sha256",
			Value:     sha256Hex([]byte(hostPromptTemplate(HostArmControl))),
			Source:    "evals.hostPromptTemplate(HostArmControl)", SourceKind: "generated",
		},
		{
			Name:      "armBPromptContract",
			Algorithm: "sha256",
			Value:     sha256Hex([]byte(hostPromptTemplate(HostArmSidecar))),
			Source:    "evals.hostPromptTemplate(HostArmSidecar)", SourceKind: "generated",
		},
		{
			Name:      "reconToolSchema",
			Algorithm: "sha256",
			Value:     sha256Hex(schema),
			Source:    "json.Marshal(sidecar.ReconMCPTool())", SourceKind: "generated",
		},
	}, nil
}
