package layout

import (
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"slices"
	"strings"
)

type lexeme struct {
	text  string
	start int
	end   int
}

// Equivalent reports how out differs from src beyond layout.
func Equivalent(src, out []byte) error {
	srcCode, srcComments, err := lexemes(src)
	if err != nil {
		return fmt.Errorf("original: %w", err)
	}

	outCode, outComments, err := lexemes(out)
	if err != nil {
		return fmt.Errorf("rewrite: %w", err)
	}

	for i := range min(len(srcCode), len(outCode)) {
		if srcCode[i].text != outCode[i].text {
			return fmt.Errorf("code differs at token %d: %q became %q", i, srcCode[i].text, outCode[i].text)
		}
	}

	if len(srcCode) != len(outCode) {
		return fmt.Errorf("code differs: %d tokens became %d", len(srcCode), len(outCode))
	}

	srcTexts := sortedTexts(srcComments)
	outTexts := sortedTexts(outComments)

	if !slices.Equal(srcTexts, outTexts) {
		return fmt.Errorf(
			"comments differ: %d became %d (%s)",
			len(srcTexts),
			len(outTexts),
			firstDifference(srcTexts, outTexts),
		)
	}

	return nil
}

// Correspondence maps code in a source file to the same code in a
// layout-only rewrite of it.
type Correspondence struct {
	srcCode     []lexeme
	outCode     []lexeme
	srcComments []lexeme
	outComments []lexeme
}

// NewCorrespondence pairs the tokens of src with those of out, which must
// be src with only its layout changed.
func NewCorrespondence(src, out []byte) (Correspondence, error) {
	if err := Equivalent(src, out); err != nil {
		return Correspondence{}, err
	}

	srcCode, srcComments, err := lexemes(src)
	if err != nil {
		return Correspondence{}, err
	}

	outCode, outComments, err := lexemes(out)
	if err != nil {
		return Correspondence{}, err
	}

	return Correspondence{
		srcCode:     srcCode,
		outCode:     outCode,
		srcComments: srcComments,
		outComments: outComments,
	}, nil
}

// Span returns the byte offsets in the rewrite of the code between offsets
// start and end of the source, which must begin and end on a token, and
// refuses a span that a comment moves into or out of.
func (c Correspondence) Span(start, end int) (int, int, error) {
	first := slices.IndexFunc(c.srcCode, func(l lexeme) bool {
		return l.start == start
	})
	last := slices.IndexFunc(c.srcCode, func(l lexeme) bool {
		return l.end == end
	})

	if first < 0 || last < first {
		return 0, 0, errors.New("the span does not begin and end on a token")
	}

	outStart, outEnd := c.outCode[first].start, c.outCode[last].end

	if !slices.Equal(
		sortedTexts(commentsWithin(c.srcComments, start, end)),
		sortedTexts(commentsWithin(c.outComments, outStart, outEnd)),
	) {
		return 0, 0, errors.New("a comment moves into or out of the span")
	}

	return outStart, outEnd, nil
}

func commentsWithin(comments []lexeme, start, end int) []lexeme {
	var inside []lexeme

	for _, c := range comments {
		if c.start >= start && c.end <= end {
			inside = append(inside, c)
		}
	}

	return inside
}

func sortedTexts(lexemes []lexeme) []string {
	texts := make([]string, 0, len(lexemes))
	for _, l := range lexemes {
		texts = append(texts, l.text)
	}

	slices.Sort(texts)

	return texts
}

func lexemes(src []byte) ([]lexeme, []lexeme, error) {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	var (
		s        scanner.Scanner
		errs     scanner.ErrorList
		code     []lexeme
		comments []lexeme
	)

	s.Init(file, src, func(pos token.Position, msg string) {
		errs.Add(pos, msg)
	}, scanner.ScanComments)

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		start := file.Offset(pos)

		switch tok {
		case token.COMMENT:
			comments = append(comments, lexeme{
				text:  strings.TrimRight(lit, " \t"),
				start: start,
				end:   start + len(lit),
			})
		case token.SEMICOLON:
		default:
			length := len(lit)
			if length == 0 {
				length = len(tok.String())
			}

			code = append(code, lexeme{
				text:  tok.String() + " " + lit,
				start: start,
				end:   start + length,
			})
		}
	}

	if errs.Len() > 0 {
		return nil, nil, errs.Err()
	}

	var kept []lexeme

	for i, l := range code {
		if l.text == token.COMMA.String()+" " && i+1 < len(code) &&
			(code[i+1].text == token.RBRACE.String()+" " || code[i+1].text == token.RPAREN.String()+" ") {
			continue
		}

		kept = append(kept, l)
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
