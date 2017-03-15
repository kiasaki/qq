package main

import (
	"fmt"
	"os"
)

type NodeType int

const (
	NTSymbol NodeType = iota
	NTKeyword
	NTNumber
	NTString
	NTList
	NTDot
	NTAssign
)

var nodeTypeNames = map[NodeType]string{
	NTSymbol:  "symbol",
	NTKeyword: "keyword",
	NTNumber:  "number",
	NTString:  "string",
	NTList:    "list",
	NTDot:     "dot",
	NTAssign:  "assign",
}

type Node struct {
	nodeType NodeType
	value    string  // For normal values
	children []*Node // For use with lists
}

func (n *Node) String() string {
	return fmt.Sprintf("[%s|%s]", nodeTypeNames[n.nodeType], n.value)
}

func NewNode(nodeType NodeType, value string) *Node {
	return &Node{
		nodeType: nodeType,
		value:    value,
	}
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

type Parser struct {
	pos    int
	source []rune
}

func parseNumber(p *Parser) *Node {
	value := ""
	c := p.source[p.pos]
	for (isNumeric(c) || c == '.') && p.pos < len(p.source) {
		value += string(c)
		p.pos++
		c = p.source[p.pos]
	}

	// Trailing dot is not part of this expression but of deref chain
	if value[len(value)-1] == '.' {
		p.pos--
		value = value[0 : len(value)-2]
	}

	return NewNode(NTNumber, value)
}

func parseSymbol(p *Parser) *Node {
	value := ""
	c := p.source[p.pos]
	for isAlphaNumeric(c) && p.pos < len(p.source) {
		value += string(c)
		p.pos++
		c = p.source[p.pos]
	}

	return NewNode(NTSymbol, value)
}

func parseKeyword(p *Parser) *Node {
	value := ""
	// Skip '`'
	p.pos++
	c := p.source[p.pos]
	for isAlphaNumeric(c) && p.pos < len(p.source) {
		value += string(c)
		p.pos++
		c = p.source[p.pos]
	}
	return NewNode(NTKeyword, value)
}

func parseEscape(p *Parser) string {
	// Skip backslach
	p.pos++
	c := p.source[p.pos]
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
		panic(fmt.Sprintf("Malformed escape ('\\') in string at position %d", p.pos))
	}
}

func parseString(p *Parser) *Node {
	startPos := p.pos
	value := ""

	// Skip opening "'"
	p.pos++

	c := p.source[p.pos]
	for c != '\'' {
		if c == '\\' {
			value += parseEscape(p)
		} else {
			value += string(c)
		}
		p.pos++
		if p.pos >= len(p.source) {
			panic(fmt.Sprintf("Unterminated string starting at position %d", startPos))
		}
		c = p.source[p.pos]
	}

	// Slip closing "'"
	p.pos++

	return NewNode(NTString, value)
}

func parseComment(p *Parser) {
	for p.pos < len(p.source) {
		c := p.source[p.pos]
		if c == '\n' || c == '\r' {
			return
		}
		p.pos++
	}
}

func parseList(p *Parser) []*Node {
	nodes := []*Node{}
	for p.pos < len(p.source) {
		c := p.source[p.pos]
		if isNumeric(c) {
			nodes = append(nodes, parseNumber(p))
		} else if isAlpha(c) {
			nodes = append(nodes, parseSymbol(p))
		} else if c == '`' {
			nodes = append(nodes, parseKeyword(p))
		} else if c == '\'' {
			nodes = append(nodes, parseString(p))
		} else if c == '.' {
			nodes = append(nodes, NewNode(NTDot, ""))
			p.pos++
		} else if c == ':' {
			nodes = append(nodes, NewNode(NTAssign, ""))
			p.pos++
		} else if c == '#' {
			parseComment(p)
		} else if isWhitespace(c) {
			// Ignore whitespace
			p.pos++
		} else {
			panic(fmt.Sprintf("Illegal character '%c' at position %d", c, p.pos))
		}
		for _, n := range nodes {
			fmt.Println(n)
		}
		fmt.Println("---------------------------")
	}
	return nodes
}

func parse(source string) []*Node {
	parser := &Parser{0, []rune(source)}
	return parseList(parser)
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
		}
	}()
	source := "o: Obj clone\no.get(`key 'asd')\n5 plus 3 # wow"
	for _, n := range parse(source) {
		fmt.Println(n)
	}
}
