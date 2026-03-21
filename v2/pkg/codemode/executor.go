package codemode

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

// ToolFunc is the callback signature for registered tools.
type ToolFunc func(args map[string]any) (any, error)

// ExecResult holds the output of a codemode execution.
type ExecResult struct {
	Value   any
	Console []string
}

// Executor runs sandboxed JavaScript with injected tool bindings.
type Executor struct {
	tools map[string]ToolFunc
}

// NewExecutor creates a new executor with the given tools.
func NewExecutor(tools map[string]ToolFunc) *Executor {
	return &Executor{tools: tools}
}

// Exec runs code in a fresh goja sandbox with injected tools and console.log.
func (e *Executor) Exec(code string) (*ExecResult, error) {
	vm := goja.New()
	var logs []string

	// Inject tools as global functions
	for name, fn := range e.tools {
		toolFn := fn // capture loop var
		vm.Set(name, func(call goja.FunctionCall) goja.Value {
			defer func() {
				if r := recover(); r != nil {
					panic(r)
				}
			}()

			arg := call.Argument(0).Export()
			args, ok := arg.(map[string]any)
			if !ok {
				panic(vm.NewGoError(fmt.Errorf("%s: expected object argument", name)))
			}

			result, err := toolFn(args)
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(result)
		})
	}

	// Inject console.log
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			parts[i] = a.String()
		}
		logs = append(logs, strings.Join(parts, " "))
		return goja.Undefined()
	})
	vm.Set("console", console)

	// Run code with panic recovery
	var val goja.Value
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("execution panic: %v", r)
			}
		}()
		val, err = vm.RunString(code)
	}()

	if err != nil {
		return nil, err
	}

	var result any
	if val != nil {
		result = val.Export()
	}

	return &ExecResult{Value: result, Console: logs}, nil
}
