package caveats

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common"
)

// CompiledCaveat is a compiled form of a caveat.
type CompiledCaveat struct {
	// env is the environment under which the CEL program was compiled.
	celEnv *cel.Env

	// ast is the AST form of the CEL program.
	ast *cel.Ast
}

// CompilationErrors is a wrapping error for containing compilation errors for a Caveat.
type CompilationErrors struct {
	error

	issues *cel.Issues
}

// CompileCaveat compiles a caveat string into a compiled caveat, or returns the compilation errors.
func CompileCaveat(env *Environment, exprString string) (*CompiledCaveat, error) {
	celEnv, err := env.asCelEnvironment()
	if err != nil {
		return nil, err
	}

	s := common.NewStringSource(exprString, "caveat")
	ast, issues := celEnv.CompileSource(s)
	if issues != nil && issues.Err() != nil {
		return nil, CompilationErrors{issues.Err(), issues}
	}

	return &CompiledCaveat{celEnv, ast}, nil
}
