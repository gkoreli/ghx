package mapengine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type TreeSitterMapper struct {
	Languages map[string]TreeSitterLanguageConfig
}

type TreeSitterLanguageConfig struct {
	TagsQuery       func(grammars.LangEntry) string
	TagKinds        map[string]Kind
	Signature       func(string, []byte, gotreesitter.Range) string
	MergeRegex      bool
	MergeRegexKinds map[Kind]bool
	TokenSource     func([]byte, *gotreesitter.Language) gotreesitter.TokenSource
}

func (m TreeSitterMapper) Map(path string, content []byte, opts Options) (Result, error) {
	opts = NormalizeOptions(opts)

	entry := m.treeSitterEntry(path)
	if entry == nil || entry.Language == nil {
		return Result{}, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, path)
	}

	lang := entry.Language()
	if lang == nil {
		return Result{}, fmt.Errorf("%w: failed to load grammar for %s", ErrUnsupportedLanguage, entry.Name)
	}

	cfg, ok := m.treeSitterConfigForEntry(*entry)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, entry.Name)
	}

	tagsQuery := cfg.resolveTagsQuery(*entry)
	if strings.TrimSpace(tagsQuery) == "" {
		return Result{}, fmt.Errorf("%w: no tags query for %s", ErrUnsupportedLanguage, entry.Name)
	}

	tagger, err := newTagger(entry, cfg, lang, tagsQuery)
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

	allTags := tagger.Tag(content)
	parents := computeTreeSitterParents(allTags)

	for i, tag := range allTags {
		symbol, ok := cfg.symbolFromTag(path, content, tag, parents[i])
		if ok {
			add(symbol)
		}
	}

	if cfg.MergeRegex {
		regexResult, _ := RegexMapper{}.Map(path, content, Options{Level: LevelCompact})
		for _, symbol := range regexResult.Symbols {
			if cfg.shouldMergeRegexSymbol(symbol) {
				add(symbol)
			}
		}
	}

	return buildResultFromSymbols(EngineTreeSitter, path, content, opts, symbols), nil
}

func (m TreeSitterMapper) treeSitterEntry(path string) *grammars.LangEntry {
	entry := grammars.DetectLanguage(path)
	if entry == nil {
		return nil
	}

	if _, ok := m.languageConfigs()[entry.Name]; ok {
		return entry
	}
	return nil
}

func (m TreeSitterMapper) treeSitterConfigForEntry(entry grammars.LangEntry) (TreeSitterLanguageConfig, bool) {
	cfg, ok := m.languageConfigs()[entry.Name]
	if !ok {
		return TreeSitterLanguageConfig{}, false
	}
	if cfg.TagsQuery == nil {
		cfg.TagsQuery = defaultTagsQuery
	}
	if cfg.TagKinds == nil {
		cfg.TagKinds = defaultTreeSitterTagKinds()
	}
	if cfg.Signature == nil {
		cfg.Signature = defaultTreeSitterSignature
	}
	if cfg.TokenSource == nil {
		cfg.TokenSource = entry.TokenSourceFactory
	}
	return cfg, true
}

func (m TreeSitterMapper) languageConfigs() map[string]TreeSitterLanguageConfig {
	if m.Languages != nil {
		return m.Languages
	}
	return defaultTreeSitterLanguageConfigs()
}

func newTagger(entry *grammars.LangEntry, cfg TreeSitterLanguageConfig, lang *gotreesitter.Language, tagsQuery string) (*gotreesitter.Tagger, error) {
	tokenSource := cfg.TokenSource
	if tokenSource == nil && entry != nil {
		tokenSource = entry.TokenSourceFactory
	}
	if tokenSource == nil {
		return gotreesitter.NewTagger(lang, tagsQuery)
	}

	return gotreesitter.NewTagger(
		lang,
		tagsQuery,
		gotreesitter.WithTaggerTokenSourceFactory(func(source []byte) gotreesitter.TokenSource {
			return tokenSource(source, lang)
		}),
	)
}

func (cfg TreeSitterLanguageConfig) resolveTagsQuery(entry grammars.LangEntry) string {
	if cfg.TagsQuery != nil {
		return cfg.TagsQuery(entry)
	}
	return defaultTagsQuery(entry)
}

func defaultTagsQuery(entry grammars.LangEntry) string {
	return grammars.ResolveTagsQuery(entry)
}

func (cfg TreeSitterLanguageConfig) symbolFromTag(path string, content []byte, tag gotreesitter.Tag, parent string) (Symbol, bool) {
	kind, ok := cfg.kindFromTag(tag.Kind)
	if !ok {
		return Symbol{}, false
	}

	line := int(tag.NameRange.StartPoint.Row) + 1
	if line <= 0 {
		line = int(tag.Range.StartPoint.Row) + 1
	}

	signature := cfg.signature(path, content, tag.Range)
	if signature == "" {
		return Symbol{}, false
	}

	return Symbol{
		Kind:      kind,
		Name:      tag.Name,
		Signature: signature,
		Line:      line,
		EndLine:   int(tag.Range.EndPoint.Row) + 1,
		Parent:    parent,
	}, true
}

// computeTreeSitterParents resolves the enclosing class/interface for each tag
// using range containment. Tags are sorted by start byte so the scope stack
// correctly handles nested types.
func computeTreeSitterParents(tags []gotreesitter.Tag) []string {
	type indexedTag struct {
		tag   gotreesitter.Tag
		index int
	}
	sorted := make([]indexedTag, len(tags))
	for i, t := range tags {
		sorted[i] = indexedTag{t, i}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].tag.Range.StartByte < sorted[j].tag.Range.StartByte
	})

	type scope struct {
		name string
		end  uint32
	}

	parents := make([]string, len(tags))
	var stack []scope

	for _, it := range sorted {
		pos := it.tag.Range.StartByte

		// pop scopes that ended before this tag's start
		n := 0
		for _, s := range stack {
			if s.end > pos {
				stack[n] = s
				n++
			}
		}
		stack = stack[:n]

		// innermost live scope is the parent
		if len(stack) > 0 {
			parents[it.index] = stack[len(stack)-1].name
		}

		// push this tag as a scope if it can contain other symbols
		if isTreeSitterContainerKind(it.tag.Kind) {
			stack = append(stack, scope{name: it.tag.Name, end: it.tag.Range.EndByte})
		}
	}

	return parents
}

func isTreeSitterContainerKind(kind string) bool {
	switch kind {
	case "definition.class", "definition.interface", "definition.module":
		return true
	}
	return false
}

func (cfg TreeSitterLanguageConfig) kindFromTag(tagKind string) (Kind, bool) {
	kinds := cfg.TagKinds
	if kinds == nil {
		kinds = defaultTreeSitterTagKinds()
	}
	kind, ok := kinds[tagKind]
	return kind, ok
}

func (cfg TreeSitterLanguageConfig) signature(path string, content []byte, r gotreesitter.Range) string {
	if cfg.Signature != nil {
		return cfg.Signature(path, content, r)
	}
	return defaultTreeSitterSignature(path, content, r)
}

func (cfg TreeSitterLanguageConfig) shouldMergeRegexSymbol(symbol Symbol) bool {
	if !cfg.MergeRegex {
		return false
	}
	if cfg.MergeRegexKinds == nil {
		return true
	}
	return cfg.MergeRegexKinds[symbol.Kind]
}

func defaultTreeSitterTagKinds() map[string]Kind {
	return map[string]Kind{
		"definition.function":    KindFunc,
		"definition.method":      KindFunc,
		"definition.constructor": KindFunc,
		"definition.class":       KindType,
		"definition.interface":   KindType,
		"definition.type":        KindType,
		"definition.constant":    KindConst,
		"definition.variable":    KindVar,
	}
}

func defaultTreeSitterLanguageConfigs() map[string]TreeSitterLanguageConfig {
	return map[string]TreeSitterLanguageConfig{
		"go": {
			MergeRegex: true,
			MergeRegexKinds: map[Kind]bool{
				KindPackage: true,
				KindImport:  true,
				KindFunc:    true,
				KindType:    true,
				KindConst:   true,
				KindVar:     true,
			},
		},
		"typescript": {
			MergeRegex: true,
		},
		"tsx": {
			MergeRegex: true,
		},
		"javascript": {
			MergeRegex: true,
		},
		"python": {
			Signature:  pythonTreeSitterSignature,
			MergeRegex: true,
		},
		"rust": {
			MergeRegex:      true,
			MergeRegexKinds: map[Kind]bool{
				KindImport: true, // `use` statements are not tagged by tree-sitter's Rust grammar
			},
		},
	}
}

func defaultTreeSitterSignature(path string, content []byte, r gotreesitter.Range) string {
	start := int(r.StartByte)
	end := int(r.EndByte)
	if start < 0 || end < start || start >= len(content) || end > len(content) {
		return ""
	}

	span := strings.TrimSpace(string(content[start:end]))
	if span == "" {
		return ""
	}

	if idx := strings.Index(span, "{"); idx >= 0 {
		return compactWhitespace(span[:idx+1])
	}

	if idx := strings.Index(span, "\n"); idx >= 0 {
		return compactWhitespace(span[:idx])
	}

	return compactWhitespace(span)
}

func pythonTreeSitterSignature(path string, content []byte, r gotreesitter.Range) string {
	span := sourceRangeText(content, r)
	if span == "" {
		return ""
	}
	if idx := strings.Index(span, ":"); idx >= 0 {
		return compactWhitespace(span[:idx+1])
	}
	return defaultTreeSitterSignature(path, content, r)
}

func sourceRangeText(content []byte, r gotreesitter.Range) string {
	start := int(r.StartByte)
	end := int(r.EndByte)
	if start < 0 || end < start || start >= len(content) || end > len(content) {
		return ""
	}
	return strings.TrimSpace(string(content[start:end]))
}

func compactWhitespace(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	return strings.Join(fields, " ")
}

func looseSymbolKey(symbol Symbol) string {
	return fmt.Sprintf("%s:%s:%d", symbol.Kind, symbol.Name, symbol.Line)
}
