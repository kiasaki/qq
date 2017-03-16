package main

import (
	"fmt"
	"strings"
)

type Lexer struct {
	pos    int
	source []rune
}

func (l *Lexer) current() rune {
	if l.pos < len(l.source) {
		return l.source[l.pos]
	}
	return rune(0)
}

func (l *Lexer) next() rune {
	l.pos++
	return l.current()
}

func (l *Lexer) hasNext() bool {
	return l.pos < len(l.source)
}

func (l *Lexer) lexNumber() *Token {
	value := ""
	c := l.current()
	for (isNumeric(c) || c == '.') && l.hasNext() {
		value += string(c)
		c = l.next()
	}

	// Trailing dot is not part of this expression but of deref chain
	if value[len(value)-1] == '.' {
		l.pos--
		value = value[0 : len(value)-2]
	}

	return NewToken(TTNumber, value)
}

func (l *Lexer) lexSymbol() *Token {
	value := ""
	c := l.current()
	for isAlphaNumeric(c) && l.hasNext() {
		value += string(c)
		c = l.next()
	}

	return NewToken(TTSymbol, value)
}

func (l *Lexer) lexKeyword() *Token {
	value := ""
	// Skip '`'
	l.next()
	c := l.current()
	for isAlphaNumeric(c) && l.hasNext() {
		value += string(c)
		c = l.next()
	}
	return NewToken(TTKeyword, value)
}

func (l *Lexer) lexEscape() string {
	// Skip backslach
	c := l.next()
	switch c {
	case '\\':
		return "\\"
	case '\'':
		return "'"
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'r':
		return "\r"
	case 'b':
		return "\b"
	case 'f':
		return "\f"
	case 'v':
		return "\v"
	case 'a':
		return "\a"
	default:
		// TODO support 0 (octal) x (2-hex) u (4-hex)
		panic(fmt.Sprintf("Malformed escape ('\\') in string at position %d", l.pos))
	}
}

func (l *Lexer) lexString() *Token {
	startPos := l.pos
	value := ""

	// Skip opening "'"
	c := l.next()
	for c != '\'' {
		if c == '\\' {
			value += l.lexEscape()
		} else {
			value += string(c)
		}
		c = l.next()
		if !l.hasNext() {
			panic(fmt.Sprintf("Unterminated string starting at position %d", startPos))
		}
	}

	// Slip closing "'"
	l.next()

	return NewToken(TTString, value)
}

func (l *Lexer) lexComment() {
	for l.hasNext() {
		c := l.current()
		if c == '\n' || c == '\r' {
			return
		}
		l.next()
	}
}

var opsC = ".:;()[]{}+-*/"
var opsT = []TokenType{
	TTDot, TTColon, TTSemiCol,
	TTLParen, TTRParen,
	TTLBrack, TTRBrack,
	TTLBrace, TTRBrace,
	TTSymbol, TTSymbol, TTSymbol, TTSymbol,
}

func (l *Lexer) lex() *Token {
	if !l.hasNext() {
		return nil
	}
	c := l.current()
	if isNumeric(c) {
		return l.lexNumber()
	} else if isAlpha(c) {
		return l.lexSymbol()
	} else if c == '`' {
		return l.lexKeyword()
	} else if c == '\'' {
		return l.lexString()
	} else if i := strings.IndexRune(opsC, c); i != -1 {
		l.next()
		return NewToken(opsT[i], string(c))
	} else if c == '#' {
		l.lexComment()
		return NewToken(TTSemiCol, ";")
	} else if isWhitespace(c) {
		// Ignore whitespace
		l.next()
		if c == '\n' || c == '\r' {
			return NewToken(TTSemiCol, ";")
		}
		return l.lex()
	} else {
		panic(fmt.Sprintf("Illegal character '%c' at position %d", c, l.pos))
	}
}

func lex(source string) []*Token {
	lexer := &Lexer{0, []rune(source)}
	tokens := []*Token{}

	token := lexer.lex()
	for token != nil {
		tokens = append(tokens, token)
		token = lexer.lex()
	}

	return tokens
}

func isNumeric(c rune) bool {
	return c >= '0' && c <= '9'
}

func isAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isAlphaNumeric(c rune) bool {
	return isAlpha(c) || isNumeric(c)
}

func isWhitespace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
