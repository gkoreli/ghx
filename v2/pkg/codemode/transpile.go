package codemode

import (
	"fmt"

	"github.com/evanw/esbuild/pkg/api"
)

// Transpile converts modern JavaScript to ES2015 compatible code using esbuild.
// Returns the original code unchanged if transpilation fails (best-effort).
func Transpile(code string) (string, error) {
	result := api.Transform(code, api.TransformOptions{
		Target: api.ES2015,
		Format: api.FormatDefault,
		Loader: api.LoaderJS,
	})
	if len(result.Errors) > 0 {
		return "", fmt.Errorf("transpile: %s", result.Errors[0].Text)
	}
	return string(result.Code), nil
}
