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

func (n *Node) MarshalJSON() ([]byte, error) {
	if n.nodeType == NTExpr || n.nodeType == NTList || n.nodeType == NTDict {
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

func NewNode(parent *Node) *Node {
	return &Node{
		parent:   parent,
		nodeType: NTExpr,
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
	if n.nodeType != NTExpr && n.nodeType != NTList && n.nodeType == NTDict {
		return n
	}

	// Group items in parens as expressions
	node := n
	originalChildren := n.children
	node.children = []*Node{}
	for _, n := range originalChildren {
		if n.tokenType == TTLParen {
			node = NewNode(node) // descend
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

	return node
}

func parse(tokens []*Token) *Node {
	node := NewNode(nil)

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
	source := "o: Obj cl0ne\no.get(`key 'as\\td')\n5 plus 3 - 1 # wow"
	tokens := lex(source)
	node := parse(tokens)
	bs, _ := json.MarshalIndent(node, "", "  ")
	fmt.Println(string(bs))
	// "github.com/k0kubun/pp"
	// pp.Println(node)
}
