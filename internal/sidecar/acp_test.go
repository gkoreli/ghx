package sidecar

import (
	"context"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

func kindPtr(k acp.ToolKind) *acp.ToolKind { return &k }

func permRequest(kind *acp.ToolKind) acp.RequestPermissionRequest {
	return acp.RequestPermissionRequest{
		SessionId: "s",
		ToolCall:  acp.ToolCallUpdate{Kind: kind},
		Options: []acp.PermissionOption{
			{OptionId: "allow", Kind: acp.PermissionOptionKindAllowOnce},
			{OptionId: "reject", Kind: acp.PermissionOptionKindRejectOnce},
		},
	}
}

func selectedOption(t *testing.T, resp acp.RequestPermissionResponse) string {
	t.Helper()
	if resp.Outcome.Selected == nil {
		t.Fatal("expected selected outcome, got cancelled")
	}
	return string(resp.Outcome.Selected.OptionId)
}

func TestRequestPermissionApprovesReadOnlyKinds(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	for _, k := range []acp.ToolKind{acp.ToolKindExecute, acp.ToolKindRead, acp.ToolKindSearch, acp.ToolKindFetch} {
		resp, err := c.RequestPermission(context.Background(), permRequest(kindPtr(k)))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "allow" {
			t.Errorf("kind %s: selected %q, want allow — the sidecar must be able to run ghx", k, got)
		}
	}
}

func TestRequestPermissionRejectsWriteKinds(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	for _, k := range []acp.ToolKind{acp.ToolKindEdit, acp.ToolKindDelete, acp.ToolKindMove} {
		resp, err := c.RequestPermission(context.Background(), permRequest(kindPtr(k)))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "reject" {
			t.Errorf("kind %s: selected %q, want reject", k, got)
		}
	}
}

func TestRequestPermissionRejectsUnknownKind(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	resp, err := c.RequestPermission(context.Background(), permRequest(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := selectedOption(t, resp); got != "reject" {
		t.Errorf("nil kind: selected %q, want reject (unclassified tools are not auto-approved)", got)
	}
}

func TestRequestPermissionCancelsWithoutMatchingOption(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	req := acp.RequestPermissionRequest{
		SessionId: "s",
		ToolCall:  acp.ToolCallUpdate{Kind: kindPtr(acp.ToolKindExecute)},
		Options: []acp.PermissionOption{
			{OptionId: "reject", Kind: acp.PermissionOptionKindRejectOnce},
		},
	}
	resp, err := c.RequestPermission(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome.Cancelled == nil {
		t.Error("expected cancelled outcome when no allow option exists for an approvable kind")
	}
}

func TestWriteAndTerminalStayRefused(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	if _, err := c.WriteTextFile(context.Background(), acp.WriteTextFileRequest{}); err == nil {
		t.Error("WriteTextFile must remain refused")
	}
	if _, err := c.ReadTextFile(context.Background(), acp.ReadTextFileRequest{}); err == nil {
		t.Error("ReadTextFile must remain refused")
	}
	if _, err := c.CreateTerminal(context.Background(), acp.CreateTerminalRequest{}); err == nil {
		t.Error("CreateTerminal must remain refused")
	}
}
