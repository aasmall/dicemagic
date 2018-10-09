package dicelang

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aasmall/word2number"
)

//Lexer steps through a source string and returns tokens
type Lexer struct {
	tokReg *tokenRegistry
	source string
	index  int
	line   int
	col    int
	tok    *AST
	cached bool
	last   *AST
	c      *word2number.Converter
}

//LexError represents an error occured during parsing of a dicelang statement.
type LexError struct {
	err  string
	Col  int
	Line int
}

func (e LexError) Error() string {
	return e.err
}

type tokenRegistry struct {
	symTable map[string]*AST
}

func (registry *tokenRegistry) token(sym string, value string, line int, col int) *AST {
	return &AST{
		Sym:          sym,
		Value:        value,
		line:         line,
		col:          col,
		BindingPower: registry.symTable[sym].BindingPower,
		nud:          registry.symTable[sym].nud,
		led:          registry.symTable[sym].led,
		std:          registry.symTable[sym].std,
	}
}

func (registry *tokenRegistry) defined(sym string) bool {
	if _, ok := registry.symTable[sym]; ok {
		return true
	}
	return false
}

func (registry *tokenRegistry) register(sym string, bp int, nud nudFn, led ledFn, std stdFn) {
	if val, ok := registry.symTable[sym]; ok {
		if nud != nil && val.nud == nil {
			val.nud = nud
		}
		if led != nil && val.led == nil {
			val.led = led
		}
		if std != nil && val.std == nil {
			val.std = std
		}
		if bp > val.BindingPower {
			val.BindingPower = bp
		}
	} else {
		registry.symTable[sym] = &AST{BindingPower: bp, nud: nud, led: led, std: std}
	}
}

// an infix token has two children, the exp on the left and the one that follows
func (registry *tokenRegistry) infix(sym string, bp int) {
	registry.register(sym, bp, nil, func(t *AST, p *Parser, left *AST) (*AST, error) {
		t.Children = append(t.Children, left)
		token, err := p.expression(t.BindingPower)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, token)
		return t, nil
	}, nil)
}

func (registry *tokenRegistry) infixLed(sym string, bp int, led ledFn) {
	registry.register(sym, bp, nil, led, nil)
}

func (registry *tokenRegistry) infixRight(sym string, bp int) {
	registry.register(sym, bp, nil, func(t *AST, p *Parser, left *AST) (*AST, error) {
		t.Children = append(t.Children, left)
		token, err := p.expression(t.BindingPower - 1)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, token)
		return t, nil
	}, nil)
}

func (registry *tokenRegistry) infixRightLed(sym string, bp int, led ledFn) {
	registry.register(sym, bp, nil, led, nil)
}

// a prefix token has a single children, the expression that follows
func (registry *tokenRegistry) prefix(sym string) {
	registry.register(sym, 0, func(t *AST, p *Parser) (*AST, error) {
		token, err := p.expression(200)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, token)
		return t, nil
	}, nil, nil)
}

func (registry *tokenRegistry) prefixNud(sym string, nud nudFn) {
	registry.register(sym, 0, nud, nil, nil)
}

func (registry *tokenRegistry) stmt(sym string, std stdFn) {
	registry.register(sym, 0, nil, nil, std)
}

func (registry *tokenRegistry) symbol(sym string) {
	registry.register(sym, 0, func(t *AST, p *Parser) (*AST, error) { return t, nil }, nil, nil)
}

func (registry *tokenRegistry) consumable(sym string) {
	registry.register(sym, 0, nil, nil, nil)
}

func (lex *Lexer) nextOperator() (*AST, error) {
	var text bytes.Buffer
	r, size := utf8.DecodeRuneInString(lex.source[lex.index:])
	col := lex.col
	lex.consumeRune(&text, r, size)

	// try to parse operators made of two characters
	var twoChar bytes.Buffer
	twoChar.WriteRune(r)
	r, size = utf8.DecodeRuneInString(lex.source[lex.index:])
	if size > 0 && isOperatorChar(r) {
		twoChar.WriteRune(r)
		if lex.tokReg.defined(twoChar.String()) {
			lex.consumeRune(&text, r, size)
			textStr := text.String()
			return lex.tokReg.token(textStr, textStr, lex.line, col), nil
		}
	}

	// single character operator
	textStr := strings.ToLower(text.String())
	if !lex.tokReg.defined(textStr) {
		return nil, LexError{fmt.Sprintf("operator not defined: %s", textStr), lex.line, col}
	}
	return lex.tokReg.token(textStr, textStr, lex.line, col), nil
}

func (lex *Lexer) nextIdent() (*AST, error) {
	var text bytes.Buffer
	col := lex.col
	r, size := utf8.DecodeRuneInString(lex.source[lex.index:])
	if r == 'd' || r == 'D' {
		r1, _ := utf8.DecodeRuneInString(lex.source[lex.index+1:])
		if unicode.IsDigit(r1) {
			return lex.nextOperator()
		}
	}
	lex.consumeRune(&text, r, size)
	for {
		r, size = utf8.DecodeRuneInString(lex.source[lex.index:])
		if size > 0 && isIdentChar(r) {
			lex.consumeRune(&text, r, size)
		} else {
			break
		}
	}
	symbol := text.String()

	if lex.tokReg.defined(symbol) {
		return lex.tokReg.token(symbol, symbol, lex.line, col), nil
	} else if found, value := convertToNumeric(lex.c, symbol); found {
		return lex.tokReg.token("(NUMBER)", strconv.Itoa(value), lex.line, col), nil
	}
	return lex.tokReg.token("(IDENT)", symbol, lex.line, col), nil
}
func convertToNumeric(c *word2number.Converter, word string) (bool, int) {
	n := c.Words2Number(word)
	if n == 0 {
		return false, 0
	}
	return true, int(n)
}
func (lex *Lexer) next() (*AST, error) {
	// invalidate peekable cache
	lex.cached = false

	tmpIndex := -1
	for lex.index != tmpIndex {
		tmpIndex = lex.index
		lex.consumeWhitespace()
		lex.consumeComments()
	}

	// end of file
	if len(lex.source[lex.index:]) == 0 {
		return lex.tokReg.token("(EOF)", "EOF", lex.line, lex.col), nil
	}

	var text bytes.Buffer
	r, size := utf8.DecodeRuneInString(lex.source[lex.index:])
	for size > 0 {
		if isFirstIdentChar(r) { // parse identifiers/keywords
			return lex.nextIdent()
		} else if unicode.IsDigit(r) { // parse numbers
			col := lex.col
			lex.consumeRune(&text, r, size)
			for {
				r, size = utf8.DecodeRuneInString(lex.source[lex.index:])
				if size > 0 && unicode.IsDigit(r) {
					lex.consumeRune(&text, r, size)
				} else {
					break
				}
			}
			if size > 0 && r == '.' {
				lex.consumeRune(&text, r, size)
				for {
					r, size = utf8.DecodeRuneInString(lex.source[lex.index:])
					if size > 0 && unicode.IsDigit(r) {
						lex.consumeRune(&text, r, size)
					} else {
						break
					}
				}
			}
			return lex.tokReg.token("(NUMBER)", text.String(), lex.line, col), nil
		} else if r == '\n' {
			lex.line++
			lex.consumeRune(&text, r, size)
			return lex.tokReg.token("(NEWLINE)", "\n", lex.line-1, lex.col), nil

		} else if isOperatorChar(r) { // parse operators
			return lex.nextOperator()
		} else {
			break
		}
	}
	panic(fmt.Sprint("INVALID CHARACTER ", lex.line, lex.col))
}

func isSpaceNotNewline(r rune) bool {
	switch r {
	case '\t', '\v', '\f', '\r', ' ', 0x85, 0xA0:
		return true
	}
	return false
}
func (lex *Lexer) consumeWhitespace() {

	r, size := utf8.DecodeRuneInString(lex.source[lex.index:])
	for size > 0 && isSpaceNotNewline(r) {
		lex.col++
		lex.index += size
		r, size = utf8.DecodeRuneInString(lex.source[lex.index:])
	}
}

func (lex *Lexer) consumeComments() {
	r, size := utf8.DecodeRuneInString(lex.source[lex.index:])
	if r == '#' {
		for size > 0 && r != '\n' {
			lex.col++
			lex.index += size
			r, size = utf8.DecodeRuneInString(lex.source[lex.index:])
		}
	}
}

func (lex *Lexer) consumeRune(text *bytes.Buffer, r rune, size int) {
	text.WriteRune(r)
	lex.col++
	lex.index += size
}

func (lex *Lexer) peek() (*AST, error) {
	if lex.cached {
		return lex.tok, nil
	}
	// save current state
	index := lex.index
	line := lex.line
	col := lex.col

	// get token and cache it
	nextToken, err := lex.next()
	if err != nil {
		return nil, err
	}
	lex.tok = nextToken
	lex.cached = true

	// restore state
	lex.index = index
	lex.line = line
	lex.col = col

	return nextToken, nil
}

//NewLexer creates a new Lexer, initializes the word2number converter and token registry.
func NewLexer(source string) *Lexer {
	c, _ := word2number.NewConverter("en")
	return &Lexer{tokReg: getTokenRegistry(), source: source, index: 0, line: 1, col: 1, c: c}
}

func getTokenRegistry() *tokenRegistry {
	t := &tokenRegistry{symTable: make(map[string]*AST)}

	t.symbol("(NUMBER)")

	t.consumable(")")
	t.consumable(",")
	t.consumable("and")
	t.consumable("else")

	t.consumable("(rootnode)")
	t.consumable("(EOF)")
	t.consumable("{")
	t.consumable("}")
	t.consumable("roll")
	t.consumable("(NEWLINE)")

	t.infix("+", 50)
	t.infix("-", 50)

	t.infix("*", 60)
	t.infix("/", 60)
	t.infix("^", 70)
	t.infix("d", 80)

	t.infixLed("-L", 80, func(t *AST, p *Parser, left *AST) (*AST, error) {
		next, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}
		if next.Sym == "(NUMBER)" {
			token, err := p.expression(t.BindingPower)
			if err != nil {
				return nil, err
			}
			t.Children = append(t.Children, token)
		} else {
			t.Children = append(t.Children, p.lexer.tokReg.token("(NUMBER)", "1", p.lexer.line, p.lexer.col))
		}
		left.Children = append(left.Children, t)
		return left, nil
	})
	t.infixLed("-H", 80, func(t *AST, p *Parser, left *AST) (*AST, error) {
		next, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}
		if next.Sym == "(NUMBER)" {
			token, err := p.expression(t.BindingPower)
			if err != nil {
				return nil, err
			}
			t.Children = append(t.Children, token)
		} else {
			t.Children = append(t.Children, p.lexer.tokReg.token("(NUMBER)", "1", p.lexer.line, p.lexer.col))
		}
		left.Children = append(left.Children, t)
		return left, nil
	})

	t.infix("mod", 95)

	t.infix("<", 30)
	t.infix(">", 30)
	t.infix("<=", 30)
	t.infix(">=", 30)
	t.infix("==", 30)
	t.infix("!=", 30)

	t.infixLed("(IDENT)", 300, func(t *AST, p *Parser, left *AST) (*AST, error) {
		t.Value = strings.Title(t.Value)
		left.Children = append(left.Children, t)
		return left, nil
	})

	t.infixLed("if", 20, func(t *AST, p *Parser, left *AST) (*AST, error) {
		cond, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, cond)
		p.advance("else")
		t.Children = append(t.Children, left)
		token, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, token)
		return t, nil
	})

	t.infixLed("(", 90, func(token *AST, p *Parser, left *AST) (*AST, error) {
		token.Children = append(token.Children, left)
		t, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}
		if t.Sym != ")" {
			for {
				exp, err := p.expression(0)
				if err != nil {
					return nil, err
				}
				token.Children = append(token.Children, exp)
				token, err := p.lexer.peek()
				if err != nil {
					return nil, err
				}
				if token.Sym != "," {
					break
				}
				p.advance(",")
			}
			p.advance(")")
		} else {
			p.advance(")")
		}
		return token, nil
	})

	t.infixLed("and", 25, func(t *AST, p *Parser, left *AST) (*AST, error) {
		left.Children = append(left.Children, t.Children...)
		return left, nil
	})
	t.infixLed(",", 25, func(t *AST, p *Parser, left *AST) (*AST, error) {
		left.Children = append(left.Children, t.Children...)
		return left, nil
	})
	t.prefix("-")

	t.prefixNud("(", func(t *AST, p *Parser) (*AST, error) {
		next, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}
		if next.Sym != ")" {
			for {
				next, err := p.lexer.peek()
				if err != nil {
					return nil, err
				}
				if next.Sym == ")" {
					break
				}
				token, err := p.expression(0)
				if err != nil {
					return nil, err
				}
				t.Children = append(t.Children, token)
			}
		}
		p.advance(")")
		return t.Children[0], nil
	})

	t.stmt("if", func(t *AST, p *Parser) (*AST, error) {
		token, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, token)
		block, err := p.block()
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, block)
		next, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}
		if next.Value == "else" {
			p.lexer.next()
			next, err = p.lexer.peek()
			if err != nil {
				return nil, err
			}
			if next.Value == "if" {
				stmt, err := p.Statement()
				if err != nil {
					return nil, err
				}
				t.Children = append(t.Children, stmt)
			} else {
				block, err := p.block()
				if err != nil {
					return nil, err
				}
				t.Children = append(t.Children, block)
			}
		}
		return t, nil
	})

	t.stmt("(NEWLINE)", func(t *AST, p *Parser) (*AST, error) {
		next, err := p.lexer.peek()
		if err != nil {
			return nil, err
		}
		for next.Sym == "(NEWLINE)" {
			p.advance("(NEWLINE)")
			next, err = p.lexer.peek()
			if err != nil {
				return nil, err
			}
		}

		if next.Sym == "(EOF)" {
			p.advance("(EOF)")
			return p.lexer.tokReg.token("(EOF)", "EOF", p.lexer.line, p.lexer.col), nil
		}
		return p.Statement()
	})

	t.stmt("roll", func(t *AST, p *Parser) (*AST, error) {
		stmt, err := p.Statement()
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, stmt)
		return t, nil
	})

	t.stmt("{", func(t *AST, p *Parser) (*AST, error) {
		stmts, _, err := p.Statements()
		if err != nil {
			return nil, err
		}
		t.Children = append(t.Children, stmts...)
		p.advance("}")
		return t, nil
	})

	return t
}

func isFirstIdentChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r == '_')
}

func isIdentChar(r rune) bool {
	return isFirstIdentChar(r) || unicode.IsDigit(r)
}

func isOperatorChar(r rune) bool {
	operators := "^*()-+=/?.,:;\"|/{}[]><dDLH"
	for _, c := range operators {
		if c == r {
			return true
		}
	}
	return false
}
