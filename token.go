package main

import "fmt"

type TokenType int

const (
	TTSymbol TokenType = iota
	TTKeyword
	TTNumber
	TTString
	TTList
	TTDot
	TTColon
	TTSemiCol
	TTLParen
	TTRParen
	TTLBrack
	TTRBrack
	TTLBrace
	TTRBrace
)

var tokenTypeNames = map[TokenType]string{
	TTSymbol:  "symbol",
	TTKeyword: "keyword",
	TTNumber:  "number",
	TTString:  "string",
	TTList:    "list",
	TTDot:     "dot",
	TTColon:   "colon",
	TTSemiCol: "semicolon",
	TTLParen:  "lparen",
	TTRParen:  "rparen",
	TTLBrack:  "lbrack",
	TTRBrack:  "rbrack",
	TTLBrace:  "lbrace",
	TTRBrace:  "rbrace",
}

type Token struct {
	tokenType TokenType
	value     string
}

func (n *Token) String() string {
	return fmt.Sprintf("[%s|%s]", tokenTypeNames[n.tokenType], n.value)
}

func NewToken(tokenType TokenType, value string) *Token {
	return &Token{
		tokenType: tokenType,
		value:     value,
	}
}
