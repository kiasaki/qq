package main

import (
	"encoding/json"
	"fmt"
)

type NodeType int

const (
	NTToken NodeType = iota
	NTExpr
	NTList
	NTDict
	NTDeref
	NTAssign
)

var nodeTypeNames = map[NodeType]string{
	NTToken:  "token",
	NTExpr:   "expr",
	NTList:   "list",
	NTDict:   "dict",
	NTDeref:  "deref",
	NTAssign: "assign",
}

type Node struct {
	// Meta
	parent   *Node
	nodeType NodeType

	// Leaf
	tokenType TokenType
	value     string

	// Branch
	children []*Node
}

func (n *Node) IsTerminal() bool {
	return n.nodeType != NTExpr && n.nodeType != NTList && n.nodeType != NTDict
}

func (n *Node) MarshalJSON() ([]byte, error) {
	if !n.IsTerminal() {
		return json.Marshal(map[string]interface{}{
			"type":     nodeTypeNames[n.nodeType],
			"children": n.children,
		})
	} else {
		return json.Marshal(map[string]string{
			"type":      nodeTypeNames[n.nodeType],
			"tokenType": tokenTypeNames[n.tokenType],
			"value":     n.value,
		})
	}
}

func (n *Node) indentedPrint(indent string) string {
	if n.IsTerminal() {
		if n.nodeType == NTToken {
			return indent + "|" + tokenTypeNames[n.tokenType] + "|" + n.value + "|" + "\n"
		} else {
			return indent + "|" + nodeTypeNames[n.nodeType] + "|" + "\n"
		}
	} else {
		out := indent + "(" + "\n"
		for _, n := range n.children {
			out += n.indentedPrint(indent + "  ")
		}
		out += indent + ")" + "\n"
		return out
	}
}

func (n *Node) Print() string {
	return n.indentedPrint("")
}

func NewNode(parent *Node, nodeType NodeType) *Node {
	return &Node{
		parent:   parent,
		nodeType: nodeType,
		children: []*Node{},
	}
}

func NewNodeFromToken(parent *Node, token *Token) *Node {
	return &Node{
		parent:    parent,
		nodeType:  NTToken,
		tokenType: token.tokenType,
		value:     token.value,
	}
}

func parseNode(n *Node) *Node {
	if n.IsTerminal() {
		return n
	}

	// Group items in parens as expressions
	node := n
	originalChildren := n.children
	node.children = []*Node{}
	for _, n := range originalChildren {
		if n.tokenType == TTLParen {
			node = NewNode(node, NTExpr) // descend
			node.parent.children = append(node.parent.children, node)
			continue
		}
		if n.tokenType == TTRParen {
			if node.parent == nil {
				panic("Superfluous closing parens")
			}
			node = node.parent // ascend
			continue
		}
		node.children = append(node.children, n)
	}

	// Split expressions on `;`
	originalChildren = node.children
	node.children = []*Node{}
	expr := NewNode(node, NTExpr)
	node.children = append(node.children, expr)
	for _, n := range originalChildren {
		if n.tokenType == TTSemiCol {
			// Start a new expr
			expr = NewNode(node, NTExpr)
			node.children = append(node.children, expr)
			continue
		}

		if n.IsTerminal() {
			expr.children = append(expr.children, n)
		} else {
			expr.children = append(expr.children, parseNode(n))
		}
	}
	// remove un-necessary nesting
	if len(node.children) == 1 {
		node.children = expr.children
	}
	// strip empty exprs from prefixed `;`
	for len(node.children[0].children) == 0 {
		node.children = node.children[1:]
	}
	// strip empty exprs from trailing `;`
	for len(node.children[len(node.children)-1].children) == 0 {
		node.children = node.children[:len(node.children)-2]
	}
	// un-nest single expression groupings
	for i, n := range node.children {
		if len(n.children) == 1 {
			node.children[i] = n.children[0]
		}
	}

	return node
}

func parse(tokens []*Token) *Node {
	node := NewNode(nil, NTExpr)

	// Convert tokens to AST nodes
	for _, t := range tokens {
		node.children = append(node.children, NewNodeFromToken(node, t))
	}

	// Start recursively parsing tokens into actual expressions
	return parseNode(node)
}

func main() {
	/*
		defer func() {
			if err := recover(); err != nil {
				fmt.Println("Error: ", err)
				os.Exit(1)
			}
		}()
	*/
	source := "o: Obj cl0ne\no.get(`key; d 'as\\td')\n5 plus 3 - 1 # wow"
	tokens := lex(source)
	node := parse(tokens)
	bs, _ := json.MarshalIndent(node, "", "  ")
	fmt.Println(string(bs))
	fmt.Println(node.Print())
	// "github.com/k0kubun/pp"
	// pp.Println(node)
}
