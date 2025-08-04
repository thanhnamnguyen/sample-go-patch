package patch

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

func (p *Patcher) patchTypeDef(id *ast.Ident, obj types.Object) {
	fieldType, ok := p.fieldTypes[obj]
	if !ok {
		return
	}

	castDecl := func(v *ast.Field) bool {
		switch t := v.Type.(type) {
		case *ast.Ident:
			t.Name = fieldType
			return true
		case *ast.ArrayType:
			v.Type = &ast.Ident{
				Name: fieldType,
			}
			return true
		case *ast.StarExpr:
			v.Type = &ast.Ident{
				Name: fieldType,
			}
			return true
		default:
			return false
		}
	}

	// Cast Field definition
	if id.Obj != nil && id.Obj.Decl != nil {
		v, ok := id.Obj.Decl.(*ast.Field)
		if !ok {
			log.Printf("Warning: fieldType declared for non-field object: %v `%s`", obj, fieldType)
			return
		}
		if !castDecl(v) {
			log.Printf("Warning: unsupported fieldType type: %T `%s`", v.Type, fieldType)
		}
		return
	}
	switch obj.Type().(type) {
	// Cast Getter signature
	case *types.Signature:
		parent := p.findParentNode(id)
		n, ok := parent.(*ast.FuncDecl)
		if !ok {
			log.Printf("Warning: unexpected type for getter: %v `%T`", obj, parent)
			break
		}
		if l := len(n.Type.Results.List); l != 1 {
			log.Printf("Warning: unexpected return count for getter: %v `%d`", obj, l)
			return
		}
		if !castDecl(n.Type.Results.List[0]) {
			log.Printf("Warning: unsupported fieldType type: %T `%s`", n.Type.Results.List[0].Type, fieldType)
		}

		// Patch function body return statements
		p.patchFunctionBody(n, fieldType)
		return
	}
}

// patchFunctionBody updates return statements in a function body to match the new return type
func (p *Patcher) patchFunctionBody(fn *ast.FuncDecl, newType string) {
	if fn.Body == nil {
		return
	}

	// Get the original return type from the function signature
	var originalType string
	if len(fn.Type.Results.List) > 0 {
		switch t := fn.Type.Results.List[0].Type.(type) {
		case *ast.Ident:
			originalType = t.Name
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				originalType = "*" + ident.Name
			}
		}
	}

	// Traverse the function body to find and update return statements
	astutil.Apply(fn.Body, func(cursor *astutil.Cursor) bool {
		if retStmt, ok := cursor.Node().(*ast.ReturnStmt); ok {
			for i, result := range retStmt.Results {
				log.Printf("result: %v", result)
				retStmt.Results[i] = p.convertReturnValue(result, originalType, newType)
			}
		}
		return true
	}, nil)
}

// convertReturnValue converts a return value to match the new return type
func (p *Patcher) convertReturnValue(expr ast.Expr, originalType, newType string) ast.Expr {
	// Handle zero value conversions
	if isZeroValue(expr, originalType) {
		return createZeroValue(newType)
	}

	// Handle type conversions for non-zero values
	if needsTypeConversion(originalType, newType) {
		return &ast.CallExpr{
			Fun: &ast.Ident{
				Name: newType,
			},
			Args: []ast.Expr{expr},
		}
	}

	return expr
}

// isZeroValue checks if an expression represents a zero value for the given type
func isZeroValue(expr ast.Expr, typ string) bool {
	switch lit := expr.(type) {
	case *ast.BasicLit:
		switch typ {
		case "string":
			return lit.Value == `""`
		case "int", "int8", "int16", "int32", "int64":
			return lit.Value == "0"
		case "uint", "uint8", "uint16", "uint32", "uint64":
			return lit.Value == "0"
		case "float32", "float64":
			return lit.Value == "0" || lit.Value == "0.0"
		case "bool":
			return lit.Value == "false"
		}
	// if return nil, return true
	case *ast.Ident:
		log.Printf("check originalType: %v", lit)
		return lit.Name == "nil"
	}
	return false
}

// createZeroValue creates an appropriate zero value for the given type
func createZeroValue(typ string) ast.Expr {
	switch typ {
	case "string":
		return &ast.BasicLit{
			Kind:  token.STRING,
			Value: `""`,
		}
	case "int", "int8", "int16", "int32", "int64":
		return &ast.BasicLit{
			Kind:  token.INT,
			Value: "0",
		}
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return &ast.BasicLit{
			Kind:  token.INT,
			Value: "0",
		}
	case "float32", "float64":
		return &ast.BasicLit{
			Kind:  token.FLOAT,
			Value: "0.0",
		}
	case "bool":
		return &ast.Ident{
			Name: "false",
		}
	default:
		// if is a pointer, return nil
		if strings.HasPrefix(typ, "*") {
			return &ast.Ident{
				Name: "nil",
			}
		}
		// if is a custom type, return type{}
		return &ast.CompositeLit{
			Type: &ast.Ident{
				Name: typ,
			},
		}

	}
}

// needsTypeConversion checks if a type conversion is needed between two types
func needsTypeConversion(from, to string) bool {
	// If types are the same, no conversion needed
	if from == to {
		return false
	}

	// Handle pointer types
	fromIsPtr := strings.HasPrefix(from, "*")
	toIsPtr := strings.HasPrefix(to, "*")

	if fromIsPtr != toIsPtr {
		return true
	}

	// Strip pointer prefixes for comparison
	fromBase := strings.TrimPrefix(from, "*")
	toBase := strings.TrimPrefix(to, "*")

	// If base types are different, conversion is needed
	return fromBase != toBase
}

func (p *Patcher) patchTypeUsage(id *ast.Ident, obj types.Object) {
	desiredType, ok := p.fieldTypes[obj]
	if !ok {
		return
	}

	node := ast.Node(id)
	for {
		node = p.findParentNode(node)
		if node == nil {
			break
		}
		if fn, ok := node.(*ast.FuncDecl); ok {
			if fn.Name.String() == "Get"+id.Name {
				return
			}
			break
		}
	}
	var originalType string
	switch t := obj.Type().(type) {
	case *types.Basic:
		originalType = t.Name()
	case *types.Pointer:
		originalType = t.String()
		desiredType = "*" + desiredType
	case *types.Signature:
		if t.Results().Len() != 1 {
			return
		}
		originalType = t.Results().At(0).Type().String()
	}
	cast := func(as string, expr ast.Expr) ast.Expr {
		if strings.HasPrefix(as, "*") {
			as = fmt.Sprintf("(%s)", as)
		}
		return &ast.CallExpr{
			Fun: &ast.Ident{
				Name: as,
			},
			Args: []ast.Expr{expr},
		}
	}

	expr := p.findParentNode(id)

	if kv, ok := expr.(*ast.KeyValueExpr); ok {
		if kv.Key == id {
			kv.Value = cast(desiredType, kv.Value)
			return
		}
		if kv.Value == id {
			kv.Value = cast(originalType, kv.Value)
			return
		}
		return
	}

	selectorExpr, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return
	}

	var patch func(node ast.Node)
	patch = func(node ast.Node) {
		switch nodeExpr := node.(type) {
		case *ast.StarExpr:
			desiredType = strings.TrimPrefix(desiredType, "*")
			originalType = strings.TrimPrefix(originalType, "*")
			expr = node
			patch(p.findParentNode(node))
			return
		case *ast.UnaryExpr:
			if nodeExpr.Op != token.AND {
				return
			}
			desiredType = "*" + desiredType
			originalType = "*" + originalType
			expr = node
			patch(p.findParentNode(node))
		case *ast.AssignStmt:
			if len(nodeExpr.Lhs) != len(nodeExpr.Rhs) {
				return
			}
			for i := range nodeExpr.Lhs {
				if nodeExpr.Lhs[i] == expr {
					nodeExpr.Rhs[i] = cast(desiredType, nodeExpr.Rhs[i])
					return
				}
			}
			for i := range nodeExpr.Rhs {
				if nodeExpr.Rhs[i] == expr {
					nodeExpr.Rhs[i] = cast(originalType, nodeExpr.Rhs[i])
					return
				}
			}
		case *ast.CallExpr:
			for i := range nodeExpr.Args {
				if nodeExpr.Args[i] == expr {
					nodeExpr.Args[i] = cast(originalType, nodeExpr.Args[i])
					return
				}
			}
			parent := p.findParentNode(nodeExpr)
			assign, isAssign := parent.(*ast.AssignStmt)
			if nodeExpr.Fun == expr && isAssign {
				for i := range assign.Rhs {
					if assign.Rhs[i] == nodeExpr {
						assign.Rhs[i] = cast(originalType, assign.Rhs[i])
						return
					}
				}
			}
			call, isCall := parent.(*ast.CallExpr)
			if isCall {
				for i := range call.Args {
					if call.Args[i] == nodeExpr {
						call.Args[i] = cast(originalType, call.Args[i])
						return
					}
				}
			}
			for i, v := range nodeExpr.Args {
				if v == expr {
					nodeExpr.Args[i] = cast(originalType, selectorExpr)
					return
				}
			}
		case *ast.BinaryExpr:
			if nodeExpr.X == expr {
				nodeExpr.X = cast(originalType, nodeExpr.X)
			}
			if nodeExpr.Y == expr {
				nodeExpr.Y = cast(originalType, nodeExpr.Y)
			}
		}
	}
	patch(p.findParentNode(expr))
}

func isTypeValid(typeName string) bool {
	return strings.Contains(typeName, ".") ||
		strings.Contains(typeName, "/") ||
		strings.Contains(typeName, "[]") ||
		strings.Contains(typeName, "*")
}
