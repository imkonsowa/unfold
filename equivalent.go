package main

import (
	"fmt"
	"go/scanner"
	"go/token"
	"slices"
	"strings"
)

// Equivalent reports how out differs from src beyond layout.
func Equivalent(src, out []byte) error {
	srcCode, srcComments, err := tokens(src)
	if err != nil {
		return fmt.Errorf("original: %w", err)
	}

	outCode, outComments, err := tokens(out)
	if err != nil {
		return fmt.Errorf("rewrite: %w", err)
	}

	for i := range min(len(srcCode), len(outCode)) {
		if srcCode[i] != outCode[i] {
			return fmt.Errorf("code differs at token %d: %q became %q", i, srcCode[i], outCode[i])
		}
	}

	if len(srcCode) != len(outCode) {
		return fmt.Errorf("code differs: %d tokens became %d", len(srcCode), len(outCode))
	}

	slices.Sort(srcComments)
	slices.Sort(outComments)

	if !slices.Equal(srcComments, outComments) {
		return fmt.Errorf(
			"comments differ: %d became %d (%s)",
			len(srcComments),
			len(outComments),
			firstDifference(srcComments, outComments),
		)
	}

	return nil
}

func tokens(src []byte) ([]string, []string, error) {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	var (
		s        scanner.Scanner
		errs     scanner.ErrorList
		code     []string
		comments []string
	)

	s.Init(file, src, func(pos token.Position, msg string) {
		errs.Add(pos, msg)
	}, scanner.ScanComments)

	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		switch tok {
		case token.COMMENT:
			comments = append(comments, strings.TrimRight(lit, " \t"))
		case token.SEMICOLON:
		default:
			code = append(code, tok.String()+" "+lit)
		}
	}

	if errs.Len() > 0 {
		return nil, nil, errs.Err()
	}

	var kept []string

	for i, t := range code {
		if t == token.COMMA.String()+" " && i+1 < len(code) &&
			(code[i+1] == token.RBRACE.String()+" " || code[i+1] == token.RPAREN.String()+" ") {
			continue
		}

		kept = append(kept, t)
	}

	return kept, comments, nil
}

func firstDifference(a, b []string) string {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return fmt.Sprintf("%q against %q", a[i], b[i])
		}
	}

	if len(a) > len(b) {
		return fmt.Sprintf("lost %q", a[len(b)])
	}

	return fmt.Sprintf("gained %q", b[len(a)])
}
