package expression

import (
	"context"
	"fmt"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"strings"
)

// ExprEngine is an implementation of the expr-lang an expression engine.
type ExprEngine struct {
}

// Eval takes a context, an expression string, and a map of variables. It returns the result of the expression evaluation
// as an interface{} and any error encountered during the evaluation process.
// If the expression string is empty, it returns nil, nil.
// If the expression string starts with "=", the "=" character is removed from the expression.
// It then compiles the expression using expr.Compile() and returns any compilation error wrapped with an ErrWorkflowFatal error.
// It runs the compiled expression using expr.Run(), passing in the variables map, and returns the result and any evaluation error.
// If there is an evaluation error, it returns the error wrapped with an additional message.
// If the evaluation is successful, it returns the result and nil as the error.
func (e *ExprEngine) Eval(ctx context.Context, exp string, vars map[string]interface{}) (interface{}, error) {
	if len(exp) == 0 {
		return nil, nil
	}
	if exp[0] == '=' {
		exp = exp[1:]
	}
	exp = strings.TrimPrefix(exp, "=")
	ex, err := expr.Compile(exp)
	if err != nil {
		return nil, fmt.Errorf(err.Error()+": %w", &errors2.ErrWorkflowFatal{Err: err})
	}

	res, err := expr.Run(ex, vars)
	if err != nil {
		return nil, fmt.Errorf("evaluate expression: %w", err)
	}

	return res, nil

}

// GetVariables takes a context and an expression string. It trims any leading and trailing white spaces from the expression string.
// If the expression string is empty, it returns nil, nil.
// If the expression string starts with "=", the "=" character is removed from the expression.
// It then parses the expression using parser.Parse() and returns any parse error.
// It walks through the AST of the parsed expression and collects all IdentifierNode types into a slice of Variable structs.
// Finally, it returns the collected variables and nil as the error.
func (e *ExprEngine) GetVariables(ctx context.Context, exp string) ([]Variable, error) {
	exp = strings.TrimSpace(exp)
	if len(exp) == 0 {
		return nil, nil
	}
	if exp[0] == '=' {
		exp = exp[1:]
	} else {
		return nil, nil
	}
	c, err := parser.Parse(exp)
	if err != nil {
		return nil, fmt.Errorf("get variables failed to parse expression %w", err)
	}

	g := &exprVariableWalker{v: make([]Variable, 0)}
	ast.Walk(&c.Node, g)
	return g.v, nil
}

type exprVariableWalker struct {
	v []Variable
}

// Visit is called from the visitor to iterate all IdentifierNode types
func (w *exprVariableWalker) Visit(n *ast.Node) {
	switch t := (*n).(type) {
	case *ast.IdentifierNode:

		w.v = append(w.v, Variable{Name: t.Value})
	}
}

// Exit is unused in the variableWalker implementation
func (w *exprVariableWalker) Exit(_ *ast.Node) {}
