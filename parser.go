package fnk

import (
	"fmt"
	"io/ioutil"
)

func ParseFile(ast *AST, file string) (err error) {
	fileIndex, new := ast.addFile(file)

	if !new {
		return fmt.Errorf("Parsing duplicate file %q", file)
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}

	return parse(ast, string(data), fileIndex)
}

func Parse(ast *AST, data string) (err error) {
	fileIndex, _ := ast.addFile("<raw data>")

	return parse(ast, data, fileIndex)
}

func parse(ast *AST, data string, fileIndex int) (err error) {
	err = parseData(ast, data, fileIndex)
	if err != nil {
		return err
	}

	err = parseAssignment(&ast.Root, ast.Files)
	if err != nil {
		return err
	}

	return nil
}

func parseData(ast *AST, data string, fileIndex int) (err error) {
	appendItem := func(n *Node, i item) {
		n.Children = append(n.Children, &Node{
			File:   fileIndex,
			Line:   i.line,
			Col:    i.col,
			Data:   i.val,
			Type:   i.typ,
			Parent: n,
		})
	}

	appendExpression := func(n *Node) {
		n.Children = append(n.Children, &Node{
			Type:   itemExpression,
			File:   fileIndex,
			Parent: n,
		})
	}

	deleteLastChildren := func(n *Node) {
		n.Children = n.Children[:len(n.Children)-1]
	}

	ast.Root.Type = itemRoot
	ast.Root.Children = append(ast.Root.Children, &Node{
		Type: itemModule,
		Data: "Main",
		File: fileIndex,
	})
	appendExpression(ast.Root.Children[0])
	node := ast.Root.Children[0].Children[0]

	lex := lex(ast.Files[fileIndex], data)

	i := lex.nextItem()
	for i.typ != itemEOF {
		switch i.typ {
		case itemError:
			return NewParseError(ast.Files[fileIndex], i.line, i.col, i.val)
		case itemNewLine:
			// handle special newline cases
			// but always skip adding it to the ast
			break
		case itemComment:
			if node.Type == itemExpression && len(node.Children) == 0 {
				// remove empty expression, add comment, add empty expression
				deleteLastChildren(node.Parent)
				appendItem(node.Parent, i)
				appendExpression(node.Parent)

				// set current node to that new empty expression
				node = node.Parent.Children[len(node.Parent.Children)-1]
			} else {
				appendItem(node, i)
			}
		default:
			// if we are at the beginning of a line, start a new expression
			if i.col == 0 && len(node.Children) > 0 {
				// TODO check expression is terminated
				// TODO backout to top level
				// start a new expression and set that as our current node
				appendExpression(node.Parent)
				node = node.Parent.Children[len(node.Parent.Children)-1]
			}

			appendItem(node, i)
		}

		i = lex.nextItem()
	}

	// if we are left with an empty expression at the end
	if node.Type == itemExpression && len(node.Children) == 0 {
		node.Parent.Children = node.Parent.Children[:len(node.Parent.Children)-1]
	}

	return nil
}

func parseAssignment(n *Node, files []string) error {
	var name *Node = nil
	var args []*Node = []*Node{}

	for i, child := range n.Children {
		switch t := child.Type; {
		case isParent(t):
			parseAssignment(child, files)
		case t == itemIdentifier && child.Data == "=":
			if i <= 0 {
				return NewParseError(files[child.File], child.Line, child.Col, "Found assignment expression (=) but missing name identifier to the left")
			}
			if i == len(n.Children)-1 {
				return NewParseError(files[child.File], child.Line, child.Col, "Found assignment expression (=) but missing expression to its right")
			}
			n.Type = itemAssignment
			expr := n.Children[i+1:]

			name.Type = itemAssignmentName
			name.Parent = n
			n.Children = []*Node{
				name,
				&Node{
					Type:     itemAssignmentArgs,
					Children: args,
					Parent:   n,
				},
				&Node{
					Type:     itemAssignmentExpr,
					Children: expr,
					Parent:   n,
				},
			}
			return nil
		case t == itemIdentifier && i == 0:
			name = child
		case t == itemIdentifier:
			args = append(args, child)
		}
	}

	return nil
}