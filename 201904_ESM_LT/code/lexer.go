package lexer

import (
	"bytes"
	"time"

	"errors"

	"sync"

	"github.com/mk2/yon/interp/kit"
	"github.com/mk2/yon/interp/token"
)

const nilRune = rune(-1)

// START LEXER_TYPE OMIT
type stateFn func(*lexer) stateFn
type lexer struct {
	sync.Mutex
	name           string
	start          kit.Position
	pos            kit.Position
	input          kit.RuneScanner
	state          stateFn
	leftDelim      rune
	rightDelim     rune
	tokens         chan kit.Token
	buf            *bytes.Buffer
	lastToken      kit.Token
	onceAgainToken bool
}

// END LEXER_TYPE OMIT

// New returns new lexer struct instance
// START NEW_LEXER OMIT
func New(r kit.RuneScanner) kit.Lexer {
	l := &lexer{
		name:           "",
		input:          r,
		state:          nil,
		tokens:         make(chan kit.Token),
		buf:            new(bytes.Buffer),
		onceAgainToken: false,
	}
	go l.run()
	return l
}

// END NEW_LEXER OMIT

/*
================================================================================
Lexer APIs
================================================================================
*/

// NextToken returns next obtaining token
// This API is blocking.
func (l *lexer) NextToken() kit.Token {

	token := <-l.tokens

	return token
}

// GetTokenCh returns token incoming channel
func (l *lexer) GetTokens() <-chan kit.Token {

	return l.tokens
}

func (l *lexer) ReadToken() (kit.Token, error) {

	if l.onceAgainToken {

		kit.Printf("found unused last token: %+v\n", l.lastToken)

		l.Lock()
		l.onceAgainToken = false
		l.Unlock()

		if l.lastToken == nil {
			return nil, errors.New("no last read token")
		}

		return l.lastToken, nil
	}

	kit.Println("waiting for incoming token")

	select {

	case t := <-l.tokens:
		l.lastToken = t
		return t, nil

	case <-time.After(kit.LexerTimeout):
		// timeout

	}

	return nil, errors.New("no token gained")
}

func (l *lexer) UnreadToken() error {

	if l.onceAgainToken {
		return errors.New("already called UreadToken")
	}

	l.Lock()
	l.onceAgainToken = true
	l.Unlock()

	return nil
}

/*
================================================================================
Lexer private methods
================================================================================
*/

// START LEXER_RUN OMIT
func (l *lexer) run() {
	for l.state = lex; l.state != nil; {
		l.state = l.state(l)
	}
}

// END LEXER_RUN OMIT

func (l *lexer) emit(t kit.TokenType) {

	val := l.buf.String()

	if val != "<nil>" {
		l.tokens <- token.Token{
			Typ: t,
			Pos: l.start,
			Val: val,
		}
	}

	l.buf.Reset()
}

func (l *lexer) peek() rune {

	var (
		r   = l.next()
		err = l.input.UnreadRune()
	)

	if err != nil {
		return nilRune
	}

	l.pos--

	return r
}

func (l *lexer) next() rune {

	var (
		r   rune
		err error
	)

	if r, _, err = l.input.ReadRune(); err != nil {
		return nilRune
	}

	l.pos++

	return r
}

/*
================================================================================
lexer functions
================================================================================
*/

// START STATEFN_START OMIT
func lex(l *lexer) stateFn {
	switch r := l.peek(); {
	case r == '(':
		l.emit(token.TLeftParen)
		l.next()
	case r == ')':
		l.emit(token.TRightParen)
		l.next()
		// END STATEFN_START OMIT

	case r == '{':
		l.emit(token.TLeftBrace)
		l.next()

	case r == '}':
		l.emit(token.TRightBrace)
		l.next()

	case r == '[':
		l.emit(token.TLeftSquareBracket)
		l.next()

	case r == ']':
		l.emit(token.TRightSquareBracket)
		l.next()

	case r == ':':
		l.emit(token.TDblColon)
		l.next()

	case r == ';':
		l.emit(token.TSemiColon)
		l.next()

	case r == '"' || r == '`':
		l.leftDelim = r
		return lexString

	case isNumber(r):
		return lexNumber

		// START STATEFN_END OMIT
	case isSpace(r):
		return lexSpace
	case isLetter(r):
		return lexIdentifier
	case r == nilRune:
		l.emit(token.TEOF)
		return nil
	default:
		kit.Printf("no matching: %v", r)
	}
	return lex
}

// END STATEFN_END OMIT

func lexIdentifier(l *lexer) stateFn {

	for r := l.peek(); isLetter(r); r = l.peek() {
		l.buf.WriteRune(r)
		l.next()
	}

	l.emit(token.TIdentifier)

	return lex
}

func lexSpace(l *lexer) stateFn {

	for isSpace(l.peek()) {
		l.next()
	}

	l.emit(token.TSpace)

	return lex
}

func lexString(l *lexer) stateFn {

	// skip the left delimiter
	l.next()

	for r := l.peek(); r != l.leftDelim; r = l.peek() {
		l.buf.WriteRune(r)
		l.next()
	}

	// skip the right delimiter
	l.next()

	l.emit(token.TString)

	return lex
}

func lexNumber(l *lexer) stateFn {

	for r := l.peek(); isNumber(r) || r == '.'; r = l.peek() {
		l.buf.WriteRune(r)
		l.next()
	}

	l.emit(token.TNumber)

	return lex
}

/*
================================================================================
Rune check functions
================================================================================
*/

func isNumber(r rune) bool {

	return '0' <= r && r <= '9'
}

func isLetter(r rune) bool {

	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' ||
		r == '-' || r == '_' || r == '?' || r == '!' || r == '.' || r == '+' || r == '%' || r == '/' || r == '=' ||
		r == '*' || r == '^' || r == '\''
}

func isSpace(r rune) bool {

	return r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r'
}
