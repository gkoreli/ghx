package mapengine

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type RegexMapper struct{}

type regexRule struct {
	kind Kind
	re   *regexp.Regexp
	name *regexp.Regexp
}

var defaultNamePattern = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*`)

var regexRules = map[string][]regexRule{
	"go": {
		newRule(KindPackage, `^package\s+([A-Za-z_][A-Za-z0-9_]*)`),
		newRule(KindImport, `^import\s+`),
		newRule(KindFunc, `^func\s+(?:\([^)]+\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
		newRule(KindType, `^type\s+([A-Za-z_][A-Za-z0-9_]*)\b`),
		newRule(KindConst, `^const\s+(?:\(|([A-Za-z_][A-Za-z0-9_]*))`),
		newRule(KindVar, `^var\s+(?:\(|([A-Za-z_][A-Za-z0-9_]*))`),
	},
	"ts":  typeScriptRules(),
	"tsx": typeScriptRules(),
	"js":  typeScriptRules(),
	"jsx": typeScriptRules(),
	"py": {
		newRule(KindImport, `^\s*(?:import|from)\s+`),
		newRule(KindType, `^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)\b`),
		newRule(KindFunc, `^\s*(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
		newRule(KindOther, `^\s*@`),
	},
	"rs": {
		newRule(KindImport, `^(?:pub\s+)?use\s+`),
		newRule(KindFunc, `^(?:pub\s+)?(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
		newRule(KindType, `^(?:pub\s+)?(?:struct|enum|trait|type|mod)\s+([A-Za-z_][A-Za-z0-9_]*)\b`),
		newRule(KindConst, `^(?:pub\s+)?const\s+([A-Za-z_][A-Za-z0-9_]*)\b`),
	},
	"java": {
		newRule(KindImport, `^import\s+`),
		newRule(KindType, `^(?:public|private|protected|abstract|final|\s)*\s*(?:class|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)\b`),
		newRule(KindOther, `^@`),
	},
	"kt": {
		newRule(KindImport, `^import\s+`),
		newRule(KindType, `^(?:public|private|protected|internal|abstract|final|data|sealed|\s)*\s*(?:class|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)\b`),
		newRule(KindFunc, `^(?:public|private|protected|internal|suspend|\s)*\s*fun\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
		newRule(KindOther, `^@`),
	},
	"rb": {
		newRule(KindImport, `^require\s+`),
		newRule(KindType, `^(?:class|module)\s+([A-Za-z_:][A-Za-z0-9_:]*)\b`),
		newRule(KindFunc, `^\s*def\s+([A-Za-z_][A-Za-z0-9_!?=]*)\b`),
	},
}

func typeScriptRules() []regexRule {
	return []regexRule{
		newRule(KindImport, `^\s*(?:import|export\s+from|export\s+\{)`),
		newRule(KindFunc, `^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`),
		newRule(KindFunc, `^(?:export\s+)?const\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*(?::[^=]+)?=\s*(?:async\s*)?\(`),
		newRule(KindType, `^(?:export\s+)?(?:abstract\s+)?(?:class|interface|type|enum)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
		newRule(KindConst, `^(?:export\s+)?const\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
		newRule(KindVar, `^(?:export\s+)?(?:let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
	}
}

func newRule(kind Kind, pattern string) regexRule {
	return regexRule{
		kind: kind,
		re:   regexp.MustCompile(pattern),
		name: defaultNamePattern,
	}
}

func (RegexMapper) Map(path string, content []byte, opts Options) (Result, error) {
	opts = NormalizeOptions(opts)

	lines := strings.Split(string(content), "\n")
	symbols := make([]Symbol, 0, len(lines)/8)
	rules := rulesForPath(path)
	for i, line := range lines {
		if line == "" {
			continue
		}
		symbol, ok := matchLine(line, i+1, rules)
		if !ok {
			continue
		}
		if opts.Kind != "" && symbol.Kind != opts.Kind {
			continue
		}
		symbols = append(symbols, symbol)
	}

	return buildResultFromSymbols(EngineRegex, path, content, opts, symbols), nil
}

func rulesForPath(path string) []regexRule {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if rules, ok := regexRules[ext]; ok {
		return rules
	}
	return []regexRule{
		newRule(KindImport, `^\s*(?:import|export|from|require|use)\s+`),
		newRule(KindFunc, `^(?:function|func|def|fn)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
		newRule(KindType, `^(?:class|interface|type|struct|enum|trait)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
		newRule(KindConst, `^(?:const|val)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
	}
}

func matchLine(line string, lineNum int, rules []regexRule) (Symbol, bool) {
	for _, rule := range rules {
		matches := rule.re.FindStringSubmatch(line)
		if len(matches) == 0 {
			continue
		}

		name := ""
		if len(matches) > 1 {
			name = matches[1]
		}
		if name == "" {
			name = inferName(line)
		}

		return Symbol{
			Kind:      rule.kind,
			Name:      name,
			Signature: strings.TrimSpace(line),
			Line:      lineNum,
			EndLine:   lineNum,
		}, true
	}
	return Symbol{}, false
}

func inferName(line string) string {
	match := defaultNamePattern.FindString(strings.TrimSpace(line))
	return match
}

func formatSymbol(symbol Symbol, level Level) string {
	switch level {
	case LevelMinimal:
		if symbol.Name != "" {
			return fmt.Sprintf("%d: %s", symbol.Line, symbol.Name)
		}
		return fmt.Sprintf("%d: %s", symbol.Line, symbol.Signature)
	default:
		return fmt.Sprintf("%d: %s", symbol.Line, symbol.Signature)
	}
}
