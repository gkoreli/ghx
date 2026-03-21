package codemode

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestExecute_HappyPath tests basic execution with tool call and console output.
func TestExecute_HappyPath(t *testing.T) {
	tools := []Tool{{
		Name: "add",
		Func: func(args map[string]any) (any, error) {
			toFloat := func(v any) float64 {
				switch x := v.(type) {
				case float64:
					return x
				case int64:
					return float64(x)
				default:
					return 0
				}
			}
			a := toFloat(args["a"])
			b := toFloat(args["b"])
			return a + b, nil
		},
	}}
	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), `
		console.log("adding");
		return callTool("add", {a: 1, b: 2});
	`, tools)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "3" {
		t.Errorf("expected value 3, got %s", result.Value)
	}
	if len(result.Console) != 1 || result.Console[0] != "adding" {
		t.Errorf("expected console [adding], got %v", result.Console)
	}
	if len(result.Calls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(result.Calls))
	}
	if result.Calls[0].Tool != "add" {
		t.Errorf("expected tool name 'add', got %s", result.Calls[0].Tool)
	}
	if result.Calls[0].Error != "" {
		t.Errorf("expected no error in call record, got %s", result.Calls[0].Error)
	}
}

// TestExecute_Timeout tests context timeout interruption.
func TestExecute_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	exec := NewExecutor()
	_, err := exec.Execute(ctx, `while(true){}`, nil)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// TestExecute_UnregisteredTool tests ACL rejection for unregistered tools.
func TestExecute_UnregisteredTool(t *testing.T) {
	exec := NewExecutor()
	_, err := exec.Execute(context.Background(), `callTool("nope", {})`, nil)

	if err == nil {
		t.Fatal("expected error for unregistered tool, got nil")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("expected 'not registered' in error, got: %v", err)
	}
}

// TestExecute_MaxToolCalls tests max tool calls limit enforcement.
func TestExecute_MaxToolCalls(t *testing.T) {
	tools := []Tool{{
		Name: "noop",
		Func: func(args map[string]any) (any, error) { return nil, nil },
	}}
	exec := NewExecutor(WithMaxToolCalls(3))
	_, err := exec.Execute(context.Background(), `
		for (var i = 0; i < 10; i++) { callTool("noop", {}); }
	`, tools)

	if err == nil {
		t.Fatal("expected error for exceeding max tool calls, got nil")
	}
	if !strings.Contains(err.Error(), "exceeded maximum tool calls") {
		t.Errorf("expected 'exceeded maximum tool calls' in error, got: %v", err)
	}
}

// TestExecute_ConsoleCapture tests console.log, console.warn, console.error capture.
func TestExecute_ConsoleCapture(t *testing.T) {
	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), `
		console.log("info");
		console.warn("warning");
		console.error("error");
		return 42;
	`, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Console) != 3 {
		t.Errorf("expected 3 console entries, got %d", len(result.Console))
	}
	if !strings.Contains(strings.Join(result.Console, " "), "info") {
		t.Errorf("expected 'info' in console, got %v", result.Console)
	}
	if !strings.Contains(strings.Join(result.Console, " "), "warning") {
		t.Errorf("expected 'warning' in console, got %v", result.Console)
	}
	if !strings.Contains(strings.Join(result.Console, " "), "error") {
		t.Errorf("expected 'error' in console, got %v", result.Console)
	}
}

// TestExecute_ModernJS tests transpilation of modern JavaScript.
func TestExecute_ModernJS(t *testing.T) {
	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), `
		const x = [1, 2, 3];
		const doubled = x.map(n => n * 2);
		return doubled;
	`, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Value, "2") || !strings.Contains(result.Value, "4") || !strings.Contains(result.Value, "6") {
		t.Errorf("expected [2,4,6] in result, got %s", result.Value)
	}
}

// TestExecute_CodeSizeLimit tests code size limit enforcement.
func TestExecute_CodeSizeLimit(t *testing.T) {
	exec := NewExecutor(WithMaxCodeSize(10))
	_, err := exec.Execute(context.Background(), "return 'this is way too long';", nil)

	if err == nil {
		t.Fatal("expected error for code size limit, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("expected 'exceeds maximum size' in error, got: %v", err)
	}
}

// TestExecute_TopLevelReturn tests IIFE wrapping for top-level return.
func TestExecute_TopLevelReturn(t *testing.T) {
	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), `return 42;`, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "42" {
		t.Errorf("expected value 42, got %s", result.Value)
	}
}

// TestExecute_SyntaxError tests syntax error handling.
func TestExecute_SyntaxError(t *testing.T) {
	exec := NewExecutor()
	_, err := exec.Execute(context.Background(), `{{{invalid`, nil)

	if err == nil {
		t.Fatal("expected syntax error, got nil")
	}
}

// TestExecute_RuntimeError tests runtime error handling.
func TestExecute_RuntimeError(t *testing.T) {
	exec := NewExecutor()
	_, err := exec.Execute(context.Background(), `throw new Error("boom");`, nil)

	if err == nil {
		t.Fatal("expected runtime error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected 'boom' in error, got: %v", err)
	}
}

// TestExecute_MarkdownFences tests markdown fence stripping.
func TestExecute_MarkdownFences(t *testing.T) {
	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), "```javascript\nreturn 42;\n```", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != "42" {
		t.Errorf("expected value 42, got %s", result.Value)
	}
}

// TestExecute_ToolCallRecords tests ToolCallRecord tracking with duration.
func TestExecute_ToolCallRecords(t *testing.T) {
	tools := []Tool{{
		Name: "slow",
		Func: func(args map[string]any) (any, error) {
			time.Sleep(10 * time.Millisecond)
			return "done", nil
		},
	}}
	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), `
		callTool("slow", {});
		callTool("slow", {});
		return "ok";
	`, tools)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Calls) != 2 {
		t.Errorf("expected 2 tool calls, got %d", len(result.Calls))
	}
	if result.Calls[0].Duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", result.Calls[0].Duration)
	}
	if result.Calls[0].Tool != "slow" {
		t.Errorf("expected tool name 'slow', got %s", result.Calls[0].Tool)
	}
}
