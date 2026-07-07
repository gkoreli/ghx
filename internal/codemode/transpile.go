package codemode

import (
	"errors"

	"github.com/evanw/esbuild/pkg/api"
)

// Transpile converts modern JavaScript to ES2015 compatible code using esbuild.
// On a transpile/parse failure it returns the raw esbuild error (unprefixed);
// the caller adds the "transpile:" context so the prefix is not doubled.
func Transpile(code string) (string, error) {
	result := api.Transform(code, api.TransformOptions{
		Target: api.ES2015,
		Format: api.FormatDefault,
		Loader: api.LoaderTS, // Accepts both TS and JS input (ADR-0009)
	})
	if len(result.Errors) > 0 {
		return "", errors.New(result.Errors[0].Text)
	}
	return string(result.Code), nil
}
