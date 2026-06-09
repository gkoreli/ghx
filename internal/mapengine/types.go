package mapengine

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type Engine string

const (
	EngineAuto       Engine = "auto"
	EngineGoAST      Engine = "go-ast"
	EngineRegex      Engine = "regex"
	EngineTreeSitter Engine = "tree-sitter"
)

type Level string

const (
	LevelCompact  Level = "compact"
	LevelMinimal  Level = "minimal"
	LevelOutline  Level = "outline"
	LevelStandard Level = "standard"
)

type Kind string

const (
	KindFunc    Kind = "func"
	KindType    Kind = "type"
	KindImport  Kind = "import"
	KindConst   Kind = "const"
	KindVar     Kind = "var"
	KindPackage Kind = "package"
	KindOther   Kind = "other"
)

type Options struct {
	Engine Engine
	Level  Level
	Kind   Kind
}

type Mapper interface {
	Map(path string, content []byte, opts Options) (Result, error)
}

type Result struct {
	Symbols       []Symbol
	Lines         []string
	Engine        Engine
	Fallback      bool
	Warnings      []string
	OriginalChars int
	MappedChars   int
}

type Symbol struct {
	Kind      Kind
	Name      string
	Signature string
	Line      int
	EndLine   int
	Parent    string
	Doc       string
}

var ErrUnsupportedLanguage = errors.New("unsupported map language")

func NormalizeOptions(opts Options) Options {
	opts.Engine = normalizeEngine(opts.Engine)
	opts.Level = normalizeLevel(opts.Level)
	opts.Kind = normalizeKind(opts.Kind)
	return opts
}

func normalizeEngine(engine Engine) Engine {
	switch Engine(strings.ToLower(strings.TrimSpace(string(engine)))) {
	case EngineRegex:
		return EngineRegex
	case EngineTreeSitter:
		return EngineTreeSitter
	default:
		return EngineAuto
	}
}

func normalizeLevel(level Level) Level {
	switch Level(strings.ToLower(strings.TrimSpace(string(level)))) {
	case LevelOutline:
		return LevelOutline
	case LevelMinimal:
		return LevelMinimal
	case LevelStandard:
		return LevelStandard
	default:
		return LevelCompact
	}
}

func normalizeKind(kind Kind) Kind {
	switch Kind(strings.ToLower(strings.TrimSpace(string(kind)))) {
	case KindFunc:
		return KindFunc
	case KindType:
		return KindType
	case KindImport:
		return KindImport
	case KindConst:
		return KindConst
	case KindVar:
		return KindVar
	case KindPackage:
		return KindPackage
	case KindOther:
		return KindOther
	default:
		return ""
	}
}

func Map(path string, content []byte, opts Options) (Result, error) {
	opts = NormalizeOptions(opts)

	switch opts.Engine {
	case EngineRegex:
		return RegexMapper{}.Map(path, content, opts)
	case EngineTreeSitter:
		return mapTreeSitterWithFallback(path, content, opts, true)
	case EngineAuto:
		return mapAuto(path, content, opts)
	default:
		result, err := RegexMapper{}.Map(path, content, opts)
		result.Fallback = true
		result.Warnings = append(result.Warnings, "unknown mapper engine; used regex fallback")
		return result, err
	}
}

func mapAuto(path string, content []byte, opts Options) (Result, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		result, err := GoASTMapper{}.Map(path, content, opts)
		if err == nil {
			return result, nil
		}
		fallback, _ := RegexMapper{}.Map(path, content, opts)
		fallback.Fallback = true
		return fallback, nil
	case ".ts", ".tsx", ".js", ".jsx", ".py", ".rs":
		return mapTreeSitterWithFallback(path, content, opts, false)
	default:
		return RegexMapper{}.Map(path, content, opts)
	}
}

func mapTreeSitterWithFallback(path string, content []byte, opts Options, warnUnsupported bool) (Result, error) {
	result, err := TreeSitterMapper{}.Map(path, content, opts)
	if err == nil {
		return result, nil
	}

	fallback, fallbackErr := RegexMapper{}.Map(path, content, opts)
	fallback.Fallback = true
	if warnUnsupported || !errors.Is(err, ErrUnsupportedLanguage) {
		fallback.Warnings = append(fallback.Warnings, fmt.Sprintf("tree-sitter mapper failed: %v; used regex fallback", err))
	}
	return fallback, fallbackErr
}

func buildResultFromSymbols(engine Engine, path string, content []byte, opts Options, symbols []Symbol) Result {
	opts = NormalizeOptions(opts)
	result := Result{
		Engine:        engine,
		OriginalChars: len(string(content)),
	}

	if opts.Level == LevelOutline {
		lines := strings.Split(string(content), "\n")
		if len(lines) == 1 && lines[0] == "" {
			result.Lines = []string{fmt.Sprintf("%s: 0 lines", path)}
		} else {
			result.Lines = []string{fmt.Sprintf("%s: 1-%d", path, len(lines))}
		}
		result.MappedChars = len(strings.Join(result.Lines, "\n"))
		return result
	}

	sort.SliceStable(symbols, func(i, j int) bool {
		if symbols[i].Line != symbols[j].Line {
			return symbols[i].Line < symbols[j].Line
		}
		if symbols[i].EndLine != symbols[j].EndLine {
			return symbols[i].EndLine < symbols[j].EndLine
		}
		return kindSortRank(symbols[i].Kind) < kindSortRank(symbols[j].Kind)
	})

	seen := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		if opts.Kind != "" && symbol.Kind != opts.Kind {
			continue
		}
		key := symbolKey(symbol)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result.Symbols = append(result.Symbols, symbol)
		result.Lines = append(result.Lines, formatSymbol(symbol, opts.Level))
	}

	result.MappedChars = len(strings.Join(result.Lines, "\n"))
	return result
}

func symbolKey(symbol Symbol) string {
	return fmt.Sprintf("%s:%s:%d:%s", symbol.Kind, symbol.Name, symbol.Line, symbol.Signature)
}

func kindSortRank(kind Kind) int {
	switch kind {
	case KindPackage:
		return 0
	case KindImport:
		return 1
	case KindType:
		return 2
	case KindFunc:
		return 3
	case KindConst:
		return 4
	case KindVar:
		return 5
	default:
		return 9
	}
}
