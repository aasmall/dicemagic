//Credit to https://blog.gopheracademy.com/advent-2014/parsers-lexers/

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

type Token int

const (
	// Special tokens
	ILLEGAL Token = iota
	WS
	ROLL     // "roll" or "Roll"
	OPAREN   // (
	CPAREN   // )
	OPERATOR // + - * /
	D        // d or D
	// Literals
	NUMBER // Sides, Number of Dice
	IDENT  //Damage Types
	EOF
)

type RollStatement struct {
	DiceSegments []DiceSegment
}
type DiceSegment struct {
	DiceRoll         []DiceRoll
	ModifierOperator string
	Modifier         int64
	DamageType       string
}
type DiceRoll struct {
	NumberOfDice int64
	Sides        int64
}

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
			return D, string(ch)
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

func (p *Parser) Parse() (*RollStatement, error) {
	//First we’ll define the AST structure we want to return from our function:

	stmt := new(RollStatement)
	//Then we’ll make sure there’s a ROLL token. If we don’t see the token we expect then we’ll return an error to report the string we found instead.
	tok, lit := p.scanIgnoreWhitespace()
	_, err := populateRequired(tok, lit, ROLL)
	if err != nil {
		return nil, fmt.Errorf("found %q, expected ROLL", lit)
	}

	//dat parse loops
	for {
		diceSgmt := new(DiceSegment)
		diceRoll := new(DiceRoll)
		// Read in number of dice of first expression
		tok, lit := p.scanIgnoreWhitespace()
		if tok == EOF {
			break
		}
		//optional: n
		if numberOfDice, consumed := populateOptional(tok, lit, NUMBER, "1"); consumed {
			diceRoll.NumberOfDice, _ = strconv.ParseInt(numberOfDice, 10, 0)
			tok, lit = p.scanIgnoreWhitespace()
		} else {
			diceRoll.NumberOfDice, _ = strconv.ParseInt(numberOfDice, 10, 0)
		}
		//required: d
		_, err := populateRequired(tok, lit, D)
		if err != nil {
			return nil, fmt.Errorf("found %q, expected D", lit)
		} else {
			log.Debugf(ctx, "found tok %q", lit)
			tok, lit = p.scanIgnoreWhitespace()
		}
		//optional: n
		if sides, consumed := populateOptional(tok, lit, NUMBER, "0"); consumed {
			diceRoll.Sides, _ = strconv.ParseInt(sides, 10, 0)
			tok, lit = p.scanIgnoreWhitespace()
		} else {
			diceRoll.Sides, _ = strconv.ParseInt(sides, 10, 0)
		}
		//optional: OPERATOR
		if operator, consumed := populateOptional(tok, lit, OPERATOR, "+"); consumed {
			diceSgmt.ModifierOperator = operator
			tok, lit = p.scanIgnoreWhitespace()
		} else {
			diceSgmt.ModifierOperator = operator
		}
		//optional: Modifier
		if operator, consumed := populateOptional(tok, lit, NUMBER, "0"); consumed {
			diceSgmt.Modifier, _ = strconv.ParseInt(operator, 10, 0)
			tok, lit = p.scanIgnoreWhitespace()
		} else {
			diceSgmt.Modifier, _ = strconv.ParseInt(operator, 10, 0)
		}
		//optional: OPAREN
		if _, found := populateOptional(tok, lit, OPAREN, "("); found {
			tok, lit = p.scanIgnoreWhitespace()
			damageType, err := populateRequired(tok, lit, IDENT)
			if err != nil {
				return nil, fmt.Errorf("found %q, expected DamageType", lit)
			} else {
				diceSgmt.DamageType = damageType
				log.Debugf(ctx, "found tok %q", lit)
				tok, lit = p.scanIgnoreWhitespace()
			}
			_, err = populateRequired(tok, lit, CPAREN)
			if err != nil {
				return nil, fmt.Errorf("found %q, expected CPAREN", lit)
			} else {
				log.Debugf(ctx, "found tok %q", lit)
				tok, lit = p.scanIgnoreWhitespace()
			}
		}

		diceSgmt.DiceRoll = append(diceSgmt.DiceRoll, *diceRoll)
		stmt.DiceSegments = append(stmt.DiceSegments, *diceSgmt)
		log.Debugf(ctx, "loopin!")

		//after a complete nDn expression, require
		//operator followed by a nother ndn expression
		//operator followed by a single number
		//--followed by another operator
		//paren follwed by a damage type followed by a close paren or EOF
	}
	return stmt, nil
}

func populateOptional(tok Token, lit string, tokExpect Token, defaultValue string) (string, bool) {
	if tok == tokExpect {
		log.Debugf(ctx, "found tok %q", lit)
		return lit, true
	} else {
		log.Debugf(ctx, "found tok: %q. Using Default: %s", lit, defaultValue)
		return defaultValue, false
	}
}
func populateRequired(tok Token, lit string, tokExpect Token) (string, error) {
	if tok == tokExpect {
		return lit, nil
	} else {
		return "", fmt.Errorf("found %q, expected %v", lit, tokExpect)
	}
}

var ctx context.Context

func parseHandler(w http.ResponseWriter, r *http.Request) {
	ctx = appengine.NewContext(r)
	//Decode request into ParseRequest type
	parseRequest := new(ParseRequest)
	json.NewDecoder(r.Body).Decode(parseRequest)

	//Prepare Response Object
	parseResponse := new(ParseResponse)

	//Call Parser and inject response into response object
	//TODO idk, roll dice?
	parsedString, err := parseString(parseRequest.Text)
	if err != nil {
		log.Criticalf(ctx, "%v", err)
		return
	}
	parseResponse.Text = parsedString

	//Encode response into response stream
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parseResponse)
}

func parseString(text string) (string, error) {
	stmt, err := NewParser(strings.NewReader(text)).Parse()
	return fmt.Sprintf("%#v", stmt), err
}
