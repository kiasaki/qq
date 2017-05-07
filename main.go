package main

import (
	"strconv"

	"github.com/k0kubun/pp"
	"github.com/prataprc/goparsec"
)

var _, _ = pp.Println("dummy print")

// AST Types
// ------------------------------------------------

type Null struct{}
type True string
type False string
type Num string
type String string

// Terminal Parsers
// ------------------------------------------------

var nullP = parsec.Token("null", "NULL")
var trueP = parsec.Token("true", "TRUE")
var falseP = parsec.Token("false", "FALSE")

var colonP = parsec.Token(`:`, "COLON")
var commaP = parsec.Token(`,`, "COMMA")
var openSqrP = parsec.Token(`\[`, "OPENSQR")
var closeSqrP = parsec.Token(`\]`, "CLOSESQR")
var openBraceP = parsec.Token(`\{`, "OPENBRACE")
var closeBraceP = parsec.Token(`\}`, "CLOSEBRACE")

var intP = parsec.Int()
var floatP = parsec.Float()

var stringP parsec.Parser = func(s parsec.Scanner) (parsec.ParsecNode, parsec.Scanner) {
	if val, news := parsec.String()(s); val != nil {
		t := parsec.Terminal{
			Name:     "STRING",
			Value:    val.(string),
			Position: news.GetCursor(),
		}
		return &t, news
	} else {
		return nil, news
	}
}

// Non-Terminal Parsers
// ------------------------------------------------

func nodifyAll(ns []parsec.ParsecNode) parsec.ParsecNode {
	return ns
}

func nodifyNth(n int) func([]parsec.ParsecNode) parsec.ParsecNode {
	return func(ns []parsec.ParsecNode) parsec.ParsecNode {
		if ns == nil || len(ns) == 0 {
			return ns
		}
		return ns[n]
	}
}

var Y parsec.Parser
var value parsec.Parser

// values -> value | values "," value
var values = parsec.Kleene(nodifyAll, &value, commaP)

// array -> "[" values "]"
var array = parsec.And(arrayNode, openSqrP, values, closeSqrP)

// property -> string ":" value
var property = parsec.And(nodifyAll, stringP, colonP, &value)

// properties -> property | properties "," property
var properties = parsec.Kleene(propertiesNode, property, commaP)

// object -> "{" properties "}"
var object = parsec.And(nodifyNth(1), openBraceP, properties, closeBraceP)

func init() {
	// value -> null | true | false | num | string | array | object
	value = parsec.OrdChoice(valueNode, nullP, trueP, falseP, intP, floatP, stringP, &array, &object)
	// expr  -> sum
	Y = parsec.OrdChoice(nodifyNth(0), value)
}

// Nodifiers
// ------------------------------------------------

func valueNode(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) == 0 {
		return nil
	}
	switch n := ns[0].(type) {
	case *parsec.Terminal:
		switch n.Name {
		case "NULL":
			return Null{}
		case "TRUE":
			return true
		case "FALSE":
			return false
		case "FLOAT":
			num, err := strconv.ParseFloat(n.Value, 64)
			if err != nil {
				panic(err)
			}
			return num
		case "INT":
			num, err := strconv.ParseInt(n.Value, 10, 64)
			if err != nil {
				panic(err)
			}
			return num
		case "STRING":
			return n.Value[1 : len(n.Value)-1]
		}
	case []parsec.ParsecNode:
		return n
	case []interface{}:
		return n
	case map[string]interface{}:
		return n
	}
	return nil
}

func arrayNode(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) == 0 {
		return nil
	}
	arr := []interface{}{}
	for _, node := range ns[1].([]parsec.ParsecNode) {
		arr = append(arr, node)
	}
	return arr
}

func propertiesNode(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil {
		return nil
	}
	m := make(map[string]interface{})
	for _, n := range ns {
		prop := n.([]parsec.ParsecNode)
		key := prop[0].(*parsec.Terminal)
		m[key.Value[1:len(key.Value)-1]] = prop[2]
	}
	return m
}

// Main
// ------------------------------------------------

func main() {
	source := `{"asd": ["asd", true, null, 1], "no": true}`
	s := parsec.NewScanner([]byte(source))
	node, s := Y(s)
	pp.Println(node)
}
