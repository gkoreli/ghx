package mapengine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type TreeSitterMapper struct{}

func (TreeSitterMapper) Map(path string, content []byte, opts Options) (Result, error) {
	opts = NormalizeOptions(opts)

	entry := treeSitterEntry(path)
	if entry == nil || entry.Language == nil {
		return Result{}, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, path)
	}

	lang := entry.Language()
	if lang == nil {
		return Result{}, fmt.Errorf("%w: failed to load grammar for %s", ErrUnsupportedLanguage, entry.Name)
	}

	tagsQuery := grammars.ResolveTagsQuery(*entry)
	if strings.TrimSpace(tagsQuery) == "" {
		return Result{}, fmt.Errorf("%w: no tags query for %s", ErrUnsupportedLanguage, entry.Name)
	}

	tagger, err := newTagger(entry, lang, tagsQuery)
	if err != nil {
		return Result{}, err
	}

	symbols := make([]Symbol, 0)
	seen := make(map[string]struct{})
	add := func(symbol Symbol) {
		if symbol.Signature == "" {
			return
		}
		key := looseSymbolKey(symbol)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		symbols = append(symbols, symbol)
	}

	for _, tag := range tagger.Tag(content) {
		symbol, ok := symbolFromTag(path, content, tag)
		if ok {
			add(symbol)
		}
	}

	regexResult, _ := RegexMapper{}.Map(path, content, Options{Level: LevelCompact})
	for _, symbol := range regexResult.Symbols {
		add(symbol)
	}

	return buildResultFromSymbols(EngineTreeSitter, path, content, opts, symbols), nil
}

func treeSitterEntry(path string) *grammars.LangEntry {
	entry := grammars.DetectLanguage(path)
	if entry == nil {
		return nil
	}

	switch entry.Name {
	case "go", "typescript", "tsx", "javascript", "python":
		return entry
	default:
		return nil
	}
}

func newTagger(entry *grammars.LangEntry, lang *gotreesitter.Language, tagsQuery string) (*gotreesitter.Tagger, error) {
	if entry.TokenSourceFactory == nil {
		return gotreesitter.NewTagger(lang, tagsQuery)
	}

	return gotreesitter.NewTagger(
		lang,
		tagsQuery,
		gotreesitter.WithTaggerTokenSourceFactory(func(source []byte) gotreesitter.TokenSource {
			return entry.TokenSourceFactory(source, lang)
		}),
	)
}

func symbolFromTag(path string, content []byte, tag gotreesitter.Tag) (Symbol, bool) {
	kind, ok := kindFromTag(tag.Kind)
	if !ok {
		return Symbol{}, false
	}

	line := int(tag.NameRange.StartPoint.Row) + 1
	if line <= 0 {
		line = int(tag.Range.StartPoint.Row) + 1
	}

	signature := signatureFromRange(path, content, tag.Range)
	if signature == "" {
		return Symbol{}, false
	}

	return Symbol{
		Kind:      kind,
		Name:      tag.Name,
		Signature: signature,
		Line:      line,
		EndLine:   int(tag.Range.EndPoint.Row) + 1,
	}, true
}

func kindFromTag(kind string) (Kind, bool) {
	switch kind {
	case "definition.function", "definition.method", "definition.constructor":
		return KindFunc, true
	case "definition.class", "definition.interface", "definition.type":
		return KindType, true
	case "definition.constant":
		return KindConst, true
	case "definition.variable":
		return KindVar, true
	default:
		return "", false
	}
}

func signatureFromRange(path string, content []byte, r gotreesitter.Range) string {
	start := int(r.StartByte)
	end := int(r.EndByte)
	if start < 0 || end < start || start >= len(content) || end > len(content) {
		return ""
	}

	span := strings.TrimSpace(string(content[start:end]))
	if span == "" {
		return ""
	}

	if filepath.Ext(path) == ".py" {
		if idx := strings.Index(span, ":"); idx >= 0 {
			return compactWhitespace(span[:idx+1])
		}
	}

	if idx := strings.Index(span, "{"); idx >= 0 {
		return compactWhitespace(span[:idx+1])
	}

	if idx := strings.Index(span, "\n"); idx >= 0 {
		return compactWhitespace(span[:idx])
	}

	return compactWhitespace(span)
}

func compactWhitespace(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	return strings.Join(fields, " ")
}

func looseSymbolKey(symbol Symbol) string {
	return fmt.Sprintf("%s:%s:%d", symbol.Kind, symbol.Name, symbol.Line)
}
