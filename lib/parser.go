package lib

//Credit to https://blog.gopheracademy.com/advent-2014/parsers-lexers/

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/aasmall/word2number"
)

type token int

const (
	illegal token = iota
	ws
	rolltoken
	oparen        // (
	cparen        // )
	obrkt         // [
	cbrkt         // ]
	operatorToken // + - * /
	numberToken   // Sides, Number of Dice
	ident         //Damage Types
	eofToken
)

//go:generate stringer -type=Token

var eof = rune(0)

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

func (s *Scanner) scan() (tok token, lit string) {
	// Read the next rune.
	ch := s.read()

	// If we see whitespace then consume all contiguous whitespace.
	// If we see a letter then consume as an ident or reserved word.
	if isWhitespace(ch) {
		s.unread()
		return s.scanWhitespace()
	} else if isLetter(ch) {
		if ch == 'd' || ch == 'D' {
			return operatorToken, string(ch)
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
		return eofToken, ""
	case '(':
		return oparen, string(ch)
	case ')':
		return cparen, string(ch)
	case '[':
		return obrkt, string(ch)
	case ']':
		return cbrkt, string(ch)
	case '+':
		return operatorToken, string(ch)
	case '-':
		return operatorToken, string(ch)
	case '*':
		return operatorToken, string(ch)
	case '/':
		return operatorToken, string(ch)
	}

	return illegal, string(ch)
}

// scanWhitespace consumes the current rune and all contiguous whitespace.
func (s *Scanner) scanWhitespace() (tok token, lit string) {
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

	return ws, buf.String()
}

// scanIdent consumes the current rune and all contiguous ident runes.
func (s *Scanner) scanIdent() (tok token, lit string) {
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
	word := strings.ToUpper(buf.String())
	if found, n := convertToNumeric(word); found {
		return numberToken, strconv.Itoa(n)
	}
	switch word {
	case "ROLL":
		return rolltoken, buf.String()
	}

	// Otherwise return as a regular identifier.
	return ident, buf.String()
}

// scanIdent consumes the current rune and all contiguous numberic runes.
func (s *Scanner) scanNumber() (tok token, lit string) {
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
	return numberToken, buf.String()
}

// Parser represents a parser.
type Parser struct {
	s   *Scanner
	buf struct {
		tok token  // last read token
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
func (p *Parser) scan() (tok token, lit string) {
	// If we have a token on the buffer, then return it.
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit
	}

	// Otherwise read the next token from the scanner.
	tok, lit = p.s.scan()

	// Save it to the buffer in case we unscan later.
	p.buf.tok, p.buf.lit = tok, lit

	return
}

// unscan pushes the previously read token back onto the buffer.
func (p *Parser) unscan() { p.buf.n = 1 }

// scanIgnoreWhitespace scans the next non-whitespace token.
func (p *Parser) scanIgnoreWhitespace() (tok token, lit string) {
	tok, lit = p.scan()
	if tok == ws {
		tok, lit = p.scan()
	}
	return
}

//MustParse calls parse without returning an error.
//Probably only use this in tests.
func (p *Parser) MustParse() *RollExpression {
	r, _ := p.Parse()
	return r
}

//Parse populates RollExpression from p Parser
func (p *Parser) Parse() (*RollExpression, error) {
	expression := new(RollExpression)
	tok, lit := p.scanIgnoreWhitespace()
	var buffer bytes.Buffer
	_, err := populateRequired(tok, lit, rolltoken)
	if err != nil {
		return nil, fmt.Errorf("found %q, expected ROLL", lit)
	}
	buffer.WriteString("Roll ")
	//flow control
	//rollback := false
	//rollbackModifier := int64(0)
	tok, lit = illegal, ""
	evalOrder := 0
	//dat parse loops
	for {
		//create this loops objects
		segment := new(Segment)
		tok, lit = p.scanIgnoreWhitespace()
		if tok == eofToken {
			break
		}
		segment.EvaluationPriority = evalOrder
		// find OParen, decrement eval order and restart loop
		if _, found := populateOptional(tok, lit, oparen); found {
			evalOrder--
			buffer.WriteString("(")
			continue
		}
		// find CParen, increment eval order and restart loop
		if _, found := populateOptional(tok, lit, cparen); found {
			evalOrder++
			buffer.WriteString(")")
			continue
		}
		//what if I don't require brackets at all?
		if segmentType, found := populateOptional(tok, strings.Title(lit), ident); found {
			for i, e := range expression.Segments {
				if e.SegmentType == "" {
					expression.Segments[i].SegmentType = segmentType
				}
			}
			buffer.WriteString("[")
			buffer.WriteString(segmentType)
			buffer.WriteString("]")
			continue
		}
		if _, found := populateOptional(tok, lit, obrkt); found {
			//found an open bracket. Read for Segment Type (force title case)
			buffer.WriteString("[")
			tok, lit = p.scanIgnoreWhitespace()
			segmentType, err := populateRequired(tok, strings.Title(lit), ident)
			if err != nil {
				return expression, err
			}
			//found segment type, Apply to all previous non-typed segments then require close bracket
			buffer.WriteString(segmentType)
			for i, e := range expression.Segments {
				if e.SegmentType == "" {
					expression.Segments[i].SegmentType = segmentType
				}
			}
			tok, lit = p.scanIgnoreWhitespace()
			_, err = populateRequired(tok, lit, cbrkt)
			if err != nil {
				return expression, err
			}
			//found close bracket, contune.
			buffer.WriteString("]")
			continue

		}
		//optional: OPERATOR
		if operator, found := populateOptional(tok, lit, operatorToken); found {
			segment.Operator = strings.ToLower(operator)
			buffer.WriteString(segment.Operator)
			tok, lit = p.scanIgnoreWhitespace()
		} else {
			segment.Operator = "+"
		}
		//optional: Number
		if number, found := populateOptional(tok, lit, numberToken); found {
			foundNumber, _ := strconv.ParseInt(number, 10, 0)
			buffer.WriteString(number)
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
		if e.Operator == "d" {
			expression.Segments[i].EvaluationPriority = getHighestPriority(expression.Segments) - 1
		}
	}
	expression.InitialText = buffer.String()
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

func populateOptional(tok token, lit string, tokExpect token) (string, bool) {
	if tok == tokExpect {
		return lit, true
	}
	return "", false

}
func populateRequired(tok token, lit string, tokExpect token) (string, error) {
	if tok == tokExpect {
		return lit, nil
	}
	return "", fmt.Errorf("found %q, expected %v", lit, tokExpect)
}

func convertToNumeric(word string) (bool, int) {
	c, _ := word2number.NewConverter("en")
	n := c.Words2Number(word)
	if n == 0 {
		return false, 0
	}
	return true, int(n)
}
