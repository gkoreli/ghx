package codemode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// ToolFunc is the callback signature for registered tools.
type ToolFunc func(args map[string]any) (any, error)

// ToolCallRecord tracks a single tool invocation.
type ToolCallRecord struct {
	Tool     string
	Args     map[string]any
	Duration time.Duration
	Error    string // empty if successful
}

// ExecuteResult holds the output of a codemode execution.
type ExecuteResult struct {
	Value   string           // JSON-encoded return value
	Console []string         // console.log/warn/error output
	Calls   []ToolCallRecord // tool call records
}

// ExecutorOption configures an Executor.
type ExecutorOption func(*Executor)

// Executor runs sandboxed JavaScript with injected tool bindings.
type Executor struct {
	maxCodeSize    int64
	maxToolCalls   int
	callTimeout    time.Duration
}

// WithMaxCodeSize sets the maximum code size (default 64KB).
func WithMaxCodeSize(size int64) ExecutorOption {
	return func(e *Executor) { e.maxCodeSize = size }
}

// WithMaxToolCalls sets the maximum tool calls per execution (default 20).
func WithMaxToolCalls(max int) ExecutorOption {
	return func(e *Executor) { e.maxToolCalls = max }
}

// WithCallTimeout sets the timeout for individual tool calls (default 30s).
func WithCallTimeout(d time.Duration) ExecutorOption {
	return func(e *Executor) { e.callTimeout = d }
}

// NewExecutor creates a new executor with optional configuration.
func NewExecutor(opts ...ExecutorOption) *Executor {
	e := &Executor{
		maxCodeSize:  64 * 1024, // 64KB default
		maxToolCalls: 20,        // default limit
		callTimeout:  30 * time.Second,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs code in a fresh goja sandbox with injected tools.
func (e *Executor) Execute(ctx context.Context, code string, tools []Tool) (*ExecuteResult, error) {
	// Check code size
	if int64(len(code)) > e.maxCodeSize {
		return nil, fmt.Errorf("code exceeds maximum size of %d bytes", e.maxCodeSize)
	}

	// Wrap code in IIFE for top-level return support
	wrappedCode := fmt.Sprintf("(function(){%s})()", code)

	vm := goja.New()
	var logs []string
	var calls []ToolCallRecord
	callCount := 0

	// Build tool map for ACL
	toolMap := make(map[string]Tool)
	for _, t := range tools {
		toolMap[t.Name] = t
	}

	// Inject callTool binding
	vm.Set("callTool", func(call goja.FunctionCall) goja.Value {
		defer func() {
			if r := recover(); r != nil {
				panic(r)
			}
		}()

		// Check call limit
		if callCount >= e.maxToolCalls {
			panic(vm.NewGoError(fmt.Errorf("exceeded maximum tool calls (%d)", e.maxToolCalls)))
		}
		callCount++

		// Extract tool name and args
		if len(call.Arguments) < 2 {
			panic(vm.NewGoError(fmt.Errorf("callTool requires 2 arguments: name and args")))
		}

		toolName := call.Argument(0).String()
		argsVal := call.Argument(1).Export()
		args, ok := argsVal.(map[string]any)
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("callTool args must be an object")))
		}

		// ACL check
		tool, exists := toolMap[toolName]
		if !exists {
			panic(vm.NewGoError(fmt.Errorf("tool not registered: %s", toolName)))
		}

		// Execute tool with timeout
		start := time.Now()
		var result any
		var err error

		// Create a context with timeout for this tool call
		callCtx, cancel := context.WithTimeout(ctx, e.callTimeout)
		defer cancel()

		// Run tool in goroutine to allow interruption
		done := make(chan struct{})
		go func() {
			result, err = tool.Func(args)
			close(done)
		}()

		// Wait for completion or context cancellation
		select {
		case <-done:
			// Tool completed normally
		case <-callCtx.Done():
			// Context cancelled or timed out
			vm.Interrupt(fmt.Errorf("tool call timeout"))
			err = callCtx.Err()
		}

		duration := time.Since(start)

		// Record the call
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		calls = append(calls, ToolCallRecord{
			Tool:     toolName,
			Args:     args,
			Duration: duration,
			Error:    errStr,
		})

		if err != nil {
			panic(vm.NewGoError(err))
		}

		return vm.ToValue(result)
	})

	// Inject console object
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			parts[i] = a.String()
		}
		logs = append(logs, strings.Join(parts, " "))
		return goja.Undefined()
	})
	console.Set("warn", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			parts[i] = a.String()
		}
		logs = append(logs, strings.Join(parts, " "))
		return goja.Undefined()
	})
	console.Set("error", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			parts[i] = a.String()
		}
		logs = append(logs, strings.Join(parts, " "))
		return goja.Undefined()
	})
	vm.Set("console", console)

	// Execute code with panic recovery
	var val goja.Value
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("execution panic: %v", r)
			}
		}()
		val, err = vm.RunString(wrappedCode)
	}()

	if err != nil {
		return nil, err
	}

	// JSON-encode return value
	var valueStr string
	if val != nil && val != goja.Undefined() {
		exported := val.Export()
		jsonBytes, err := json.Marshal(exported)
		if err != nil {
			return nil, fmt.Errorf("failed to JSON-encode return value: %w", err)
		}
		valueStr = string(jsonBytes)
	} else {
		valueStr = "null"
	}

	return &ExecuteResult{
		Value:   valueStr,
		Console: logs,
		Calls:   calls,
	}, nil
}
