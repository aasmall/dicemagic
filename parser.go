//Credit to https://blog.gopheracademy.com/advent-2014/parsers-lexers/

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

type Token int

const (
	ILLEGAL Token = iota
	WS
	ROLL     // "roll" or "Roll"
	OPAREN   // (
	CPAREN   // )
	OBRKT    // [
	CBRKT    // ]
	OPERATOR // + - * /
	NUMBER   // Sides, Number of Dice
	IDENT    //Damage Types
	EOF
)

//go:generate stringer -type=Token

var eof = rune(0)

type ParseRequest struct {
	Text string `json:"text"`
}
type ParseResponse struct {
	Text string `json:"text"`
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}
func isNumber(ch rune) bool {
	return (unicode.IsDigit(ch))
}

// Scanner represents a lexical scanner.
type Scanner struct {
	r *bufio.Reader
}

// NewScanner returns a new instance of Scanner.
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{r: bufio.NewReader(r)}
}

// read reads the next rune from the bufferred reader.
// Returns the rune(0) if an error occurs (or io.EOF is returned).
func (s *Scanner) read() rune {
	ch, _, err := s.r.ReadRune()
	if err != nil {
		return eof
	}
	return ch
}

// unread places the previously read rune back on the reader.
func (s *Scanner) unread() { _ = s.r.UnreadRune() }

func (s *Scanner) Scan() (tok Token, lit string) {
	// Read the next rune.
	ch := s.read()

	// If we see whitespace then consume all contiguous whitespace.
	// If we see a letter then consume as an ident or reserved word.
	if isWhitespace(ch) {
		s.unread()
		return s.scanWhitespace()
	} else if isLetter(ch) {
		if ch == 'd' || ch == 'D' {
			return OPERATOR, string(ch)
		}
		s.unread()
		return s.scanIdent()
	} else if isNumber(ch) {
		s.unread()
		return s.scanNumber()
	}

	// Otherwise read the individual character.
	switch ch {
	case eof:
		return EOF, ""
	case '(':
		return OPAREN, string(ch)
	case ')':
		return CPAREN, string(ch)
	case '[':
		return OBRKT, string(ch)
	case ']':
		return CBRKT, string(ch)
	case '+':
		return OPERATOR, string(ch)
	case '-':
		return OPERATOR, string(ch)
	case '*':
		return OPERATOR, string(ch)
	case '/':
		return OPERATOR, string(ch)
	}

	return ILLEGAL, string(ch)
}

// scanWhitespace consumes the current rune and all contiguous whitespace.
func (s *Scanner) scanWhitespace() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent whitespace character into the buffer.
	// Non-whitespace characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isWhitespace(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	return WS, buf.String()
}

// scanIdent consumes the current rune and all contiguous ident runes.
func (s *Scanner) scanIdent() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent ident character into the buffer.
	// Non-ident characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isLetter(ch) {
			s.unread()
			break
		} else {
			_, _ = buf.WriteRune(ch)
		}
	}

	// If the string matches a keyword then return that keyword.
	switch strings.ToUpper(buf.String()) {
	case "ROLL":
		return ROLL, buf.String()
	}

	// Otherwise return as a regular identifier.
	return IDENT, buf.String()
}

// scanIdent consumes the current rune and all contiguous numberic runes.
func (s *Scanner) scanNumber() (tok Token, lit string) {
	// Create a buffer and read the current character into it.
	var buf bytes.Buffer
	buf.WriteRune(s.read())

	// Read every subsequent ident character into the buffer.
	// Non-ident characters and EOF will cause the loop to exit.
	for {
		if ch := s.read(); ch == eof {
			break
		} else if !isNumber(ch) {
			s.unread()
			break
		} else {
			_, _ = buf.WriteRune(ch)
		}
	}
	return NUMBER, buf.String()
}

// Parser represents a parser.
type Parser struct {
	s   *Scanner
	buf struct {
		tok Token  // last read token
		lit string // last read literal
		n   int    // buffer size (max=1)
	}
}

// NewParser returns a new instance of Parser.
func NewParser(r io.Reader) *Parser {
	return &Parser{s: NewScanner(r)}
}

// scan returns the next token from the underlying scanner.
// If a token has been unscanned then read that instead.
func (p *Parser) scan() (tok Token, lit string) {
	// If we have a token on the buffer, then return it.
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit
	}

	// Otherwise read the next token from the scanner.
	tok, lit = p.s.Scan()

	// Save it to the buffer in case we unscan later.
	p.buf.tok, p.buf.lit = tok, lit

	return
}

// unscan pushes the previously read token back onto the buffer.
func (p *Parser) unscan() { p.buf.n = 1 }

// scanIgnoreWhitespace scans the next non-whitespace token.
func (p *Parser) scanIgnoreWhitespace() (tok Token, lit string) {
	tok, lit = p.scan()
	if tok == WS {
		tok, lit = p.scan()
	}
	return
}

func (p *Parser) MustParse() *RollExpression {
	r, _ := p.Parse()
	return r
}
func (p *Parser) Parse() (*RollExpression, error) {
	expression := new(RollExpression)
	tok, lit := p.scanIgnoreWhitespace()
	_, err := populateRequired(tok, lit, ROLL)
	if err != nil {
		return nil, fmt.Errorf("found %q, expected ROLL", lit)
	}

	//flow control
	//rollback := false
	//rollbackModifier := int64(0)
	tok, lit = ILLEGAL, ""
	evalOrder := 0
	//dat parse loops
	for {
		//create this loops objects
		segment := new(Segment)
		tok, lit = p.scanIgnoreWhitespace()
		if tok == EOF {
			break
		}
		segment.EvaluationPriority = evalOrder
		// find OParen, decrement eval order and restart loop
		if _, found := populateOptional(tok, lit, OPAREN); found {
			evalOrder--
			continue
		}
		// find CParen, increment eval order and restart loop
		if _, found := populateOptional(tok, lit, CPAREN); found {
			evalOrder++
			continue
		}
		if _, found := populateOptional(tok, lit, OBRKT); found {
			//found an open bracket. Read for Segment Type (force title case)
			tok, lit = p.scanIgnoreWhitespace()
			segmentType, err := populateRequired(tok, strings.Title(lit), IDENT)
			if err != nil {
				return expression, err
			}
			//found segment type, Apply to all previous non-typed segments then require close bracket
			for i, e := range expression.Segments {
				if e.SegmentType == "" {
					expression.Segments[i].SegmentType = segmentType
				}
			}
			tok, lit = p.scanIgnoreWhitespace()
			_, err = populateRequired(tok, lit, CBRKT)
			if err != nil {
				return expression, err
			}
			//found close bracket, contune.
			continue

		}
		//optional: OPERATOR
		if operator, found := populateOptional(tok, lit, OPERATOR); found {
			segment.Operator = strings.ToUpper(operator)
			tok, lit = p.scanIgnoreWhitespace()
		} else {
			segment.Operator = "+"
		}
		//optional: Number
		if number, found := populateOptional(tok, lit, NUMBER); found {
			foundNumber, _ := strconv.ParseInt(number, 10, 0)
			segment.Number = foundNumber
		}
		expression.Segments = append(expression.Segments, *segment)
	}
	for i, e := range expression.Segments {
		if e.Operator == "*" || e.Operator == "/" {
			expression.adjustIfLowerPriority(expression.Segments[i].EvaluationPriority, -1)
			expression.Segments[i].EvaluationPriority += -1
		}
	}

	//force dice rolls to highest priority
	for i, e := range expression.Segments {
		if e.Operator == "D" {
			expression.Segments[i].EvaluationPriority = GetHighestPriority(expression.Segments) - 1
		}
	}
	return expression, nil
}

func (e *RollExpression) adjustIfLowerPriority(ifLowerThan int, adjustBy int) {
	for i, s := range e.Segments {
		if s.EvaluationPriority < ifLowerThan {
			e.Segments[i].EvaluationPriority += adjustBy
		}
	}
}
func (e *RollExpression) adjustIfHigherPriority(ifHigherThan int, adjustBy int) {
	for i, s := range e.Segments {
		if s.EvaluationPriority > ifHigherThan {
			e.Segments[i].EvaluationPriority += adjustBy
		}
	}
}

func populateOptional(tok Token, lit string, tokExpect Token) (string, bool) {
	if tok == tokExpect {
		return lit, true
	}
	return "", false

}
func populateRequired(tok Token, lit string, tokExpect Token) (string, error) {
	if tok == tokExpect {
		return lit, nil
	}
	return "", fmt.Errorf("found %q, expected %v", lit, tokExpect)
}
