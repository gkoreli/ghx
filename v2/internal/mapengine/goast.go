package mapengine

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type GoASTMapper struct{}

func (GoASTMapper) Map(path string, content []byte, opts Options) (Result, error) {
	opts = NormalizeOptions(opts)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, 0)
	if err != nil && f == nil {
		return Result{}, fmt.Errorf("%w: go/ast: %v", ErrUnsupportedLanguage, err)
	}

	symbols := goASTSymbols(fset, f, content)
	return buildResultFromSymbols(EngineGoAST, path, content, opts, symbols), nil
}

func goASTSymbols(fset *token.FileSet, f *ast.File, content []byte) []Symbol {
	var out []Symbol

	if f.Name != nil {
		p := fset.Position(f.Package)
		out = append(out, Symbol{
			Kind:      KindPackage,
			Name:      f.Name.Name,
			Signature: "package " + f.Name.Name,
			Line:      p.Line,
			EndLine:   p.Line,
		})
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			out = append(out, goGenDeclSymbols(fset, d, content)...)
		case *ast.FuncDecl:
			out = append(out, goFuncDeclSymbol(fset, d, content))
		}
	}

	return out
}

func goGenDeclSymbols(fset *token.FileSet, d *ast.GenDecl, content []byte) []Symbol {
	tok := d.Tok.String()
	pos := fset.Position(d.Pos())
	end := fset.Position(d.End())

	switch tok {
	case "import":
		sig := "import ("
		if !d.Lparen.IsValid() && len(d.Specs) == 1 {
			sig = compactWhitespace(goSlice(content, fset, d.Pos(), d.End()))
		}
		return []Symbol{{
			Kind:      KindImport,
			Name:      "import",
			Signature: sig,
			Line:      pos.Line,
			EndLine:   end.Line,
		}}

	case "type":
		var syms []Symbol
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			syms = append(syms, goTypeSymbol(fset, ts, content))
		}
		return syms

	case "const", "var":
		kind := KindConst
		if tok == "var" {
			kind = KindVar
		}
		if d.Lparen.IsValid() {
			return []Symbol{{
				Kind:      kind,
				Name:      tok,
				Signature: tok + " (",
				Line:      pos.Line,
				EndLine:   end.Line,
			}}
		}
		if len(d.Specs) == 1 {
			if vs, ok := d.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) > 0 {
				return []Symbol{{
					Kind:      kind,
					Name:      vs.Names[0].Name,
					Signature: compactWhitespace(goSlice(content, fset, d.Pos(), d.End())),
					Line:      pos.Line,
					EndLine:   end.Line,
				}}
			}
		}
	}
	return nil
}

func goTypeSymbol(fset *token.FileSet, ts *ast.TypeSpec, content []byte) Symbol {
	pos := fset.Position(ts.Name.Pos())

	rawBody := goSlice(content, fset, ts.Type.Pos(), ts.Type.End())
	var typePart string
	switch ts.Type.(type) {
	case *ast.StructType, *ast.InterfaceType:
		if idx := strings.Index(rawBody, "{"); idx >= 0 {
			typePart = compactWhitespace(rawBody[:idx+1])
		} else {
			typePart = compactWhitespace(rawBody)
		}
	default:
		typePart = compactWhitespace(rawBody)
	}

	return Symbol{
		Kind:      KindType,
		Name:      ts.Name.Name,
		Signature: "type " + ts.Name.Name + " " + typePart,
		Line:      pos.Line,
		EndLine:   fset.Position(ts.End()).Line,
	}
}

func goFuncDeclSymbol(fset *token.FileSet, d *ast.FuncDecl, content []byte) Symbol {
	pos := fset.Position(d.Pos())

	var sig string
	if d.Body != nil {
		raw := goSlice(content, fset, d.Pos(), d.Body.Lbrace+1)
		sig = compactWhitespace(raw)
	} else {
		raw := goSlice(content, fset, d.Pos(), d.End())
		sig = compactWhitespace(raw)
	}

	return Symbol{
		Kind:      KindFunc,
		Name:      d.Name.Name,
		Signature: sig,
		Line:      pos.Line,
		EndLine:   fset.Position(d.End()).Line,
	}
}

func goSlice(content []byte, fset *token.FileSet, start, end token.Pos) string {
	s := fset.Position(start).Offset
	e := fset.Position(end).Offset
	if s < 0 || e <= s || e > len(content) {
		return ""
	}
	return string(content[s:e])
}
