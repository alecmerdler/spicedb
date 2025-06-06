package caveats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/authzed/cel-go/cel"

	"github.com/authzed/spicedb/pkg/caveats/types"
)

var noMissingVars []string

func TestEvaluateCaveat(t *testing.T) {
	wetTz, err := time.LoadLocation("WET")
	require.NoError(t, err)
	tcs := []struct {
		name       string
		env        *Environment
		exprString string

		context map[string]any

		expectedError string

		expectedValue       bool
		expectedPartialExpr string
		missingVars         []string
	}{
		{
			"static expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{}),
			"true",
			map[string]any{},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"static false expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{}),
			"false",
			map[string]any{},
			"",
			false,
			"",
			noMissingVars,
		},
		{
			"static numeric expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{}),
			"1 + 2 == 3",
			map[string]any{},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"static false numeric expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{}),
			"2 - 2 == 1",
			map[string]any{},
			"",
			false,
			"",
			noMissingVars,
		},
		{
			"computed expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.IntType,
			}),
			"a + 2 == 4",
			map[string]any{
				"a": 2,
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"missing variables for expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.IntType,
			}),
			"a + 2 == 4",
			map[string]any{},
			"",
			false,
			"a + 2 == 4",
			[]string{"a"},
		},
		{
			"missing variables for right side of boolean expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.IntType,
				"b": types.Default.IntType,
			}),
			"(a == 2) || (b == 6)",
			map[string]any{
				"a": 2,
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"missing variables for left side of boolean expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.IntType,
				"b": types.Default.IntType,
			}),
			"(a == 2) || (b == 6)",
			map[string]any{
				"b": 6,
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"missing variables for both sides of boolean expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.IntType,
				"b": types.Default.IntType,
			}),
			"(a == 2) || (b == 6)",
			map[string]any{},
			"",
			false,
			"a == 2 || b == 6",
			[]string{"a", "b"},
		},
		{
			"missing variable for left side of and boolean expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.IntType,
				"b": types.Default.IntType,
			}),
			"(a == 2) && (b == 6)",
			map[string]any{
				"b": 6,
			},
			"",
			false,
			"a == 2",
			[]string{"a"},
		},
		{
			"missing variable for right side of and boolean expression",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.IntType,
				"b": types.Default.IntType,
			}),
			"(a == 2) && (b == 6)",
			map[string]any{
				"a": 2,
			},
			"",
			false,
			"b == 6",
			[]string{"b"},
		},
		{
			"map evaluation",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"m":   types.Default.MustMapType(types.Default.BooleanType),
				"idx": types.Default.StringType,
			}),
			"m[idx]",
			map[string]any{
				"m": map[string]bool{
					"1": true,
				},
				"idx": "1",
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"map dot evaluation",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"m": types.Default.MustMapType(types.Default.BooleanType),
			}),
			"m.foo",
			map[string]any{
				"m": map[string]bool{
					"foo": true,
				},
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"missing map for evaluation",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"m":   types.Default.MustMapType(types.Default.BooleanType),
				"idx": types.Default.StringType,
			}),
			"m[idx]",
			map[string]any{
				"idx": "1",
			},
			"",
			false,
			"m[idx]",
			[]string{"m"},
		},
		{
			"missing map for attribute evaluation",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"m": types.Default.MustMapType(types.Default.BooleanType),
			}),
			"m.first",
			map[string]any{},
			"",
			false,
			"m.first",
			[]string{"m"},
		},
		{
			"nested evaluation",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"metadata.l":   types.Default.MustListType(types.Default.StringType),
				"metadata.idx": types.Default.IntType,
			}),
			"metadata.l[metadata.idx] == 'hello'",
			map[string]any{
				"metadata.l":   []string{"hi", "hello", "yo"},
				"metadata.idx": 1,
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"nested evaluation with missing value",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"metadata.l":   types.Default.MustListType(types.Default.StringType),
				"metadata.idx": types.Default.IntType,
			}),
			"metadata.l[metadata.idx] == 'hello'",
			map[string]any{
				"metadata.l": []string{"hi", "hello", "yo"},
			},
			"",
			false,
			`metadata.l[metadata.idx] == "hello"`,
			[]string{"metadata.idx"},
		},
		{
			"nested evaluation with missing list",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"metadata.l":   types.Default.MustListType(types.Default.StringType),
				"metadata.idx": types.Default.IntType,
			}),
			"metadata.l[metadata.idx] == 'hello'",
			map[string]any{
				"metadata.idx": 1,
			},
			"",
			false,
			`metadata.l[metadata.idx] == "hello"`,
			[]string{"metadata.l"},
		},
		{
			"timestamp operations default to UTC",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.TimestampType,
			}),
			"a.getHours() == 9",
			map[string]any{
				"a": time.Date(2000, 10, 10, 10, 10, 10, 10, wetTz),
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"timestamp comparison",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.TimestampType,
				"b": types.Default.TimestampType,
			}),
			"a < b",
			map[string]any{
				"a": time.Date(2000, 10, 10, 10, 10, 10, 10, wetTz),
				"b": time.Date(2000, 10, 10, 10, 10, 10, 10, wetTz),
			},
			"",
			false,
			"",
			noMissingVars,
		},
		{
			"timestamp comparison 2",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"a": types.Default.TimestampType,
				"b": types.Default.TimestampType,
			}),
			"a <= b",
			map[string]any{
				"a": time.Date(2000, 10, 10, 10, 10, 10, 10, wetTz),
				"b": time.Date(2000, 10, 10, 10, 10, 10, 10, wetTz),
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"optional types not found",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"m":   types.Default.MustMapType(types.Default.BooleanType),
				"key": types.Default.StringType,
			}),
			"m[?key].orValue(true)",
			map[string]any{
				"m":   map[string]bool{"foo": true, "bar": false},
				"key": "baz",
			},
			"",
			true,
			"",
			noMissingVars,
		},
		{
			"optional types found",
			MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
				"m":   types.Default.MustMapType(types.Default.BooleanType),
				"key": types.Default.StringType,
			}),
			"m[?key].orValue(true)",
			map[string]any{
				"m":   map[string]bool{"foo": true, "bar": false},
				"key": "bar",
			},
			"",
			false,
			"",
			noMissingVars,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			compiled, err := compileCaveat(tc.env, tc.exprString)
			require.NoError(t, err)

			result, err := EvaluateCaveat(compiled, tc.context)
			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.expectedValue, result.Value())

				if tc.expectedPartialExpr != "" {
					require.True(t, result.IsPartial())

					partialValue, err := result.PartialValue()
					require.NoError(t, err)

					astExpr, err := cel.AstToString(partialValue.ast)
					require.NoError(t, err)

					require.Equal(t, tc.expectedPartialExpr, astExpr)

					vars, err := result.MissingVarNames()
					require.NoError(t, err)
					require.EqualValues(t, tc.missingVars, vars)
				} else {
					require.False(t, result.IsPartial())
					_, partialErr := result.PartialValue()
					require.Error(t, partialErr)
					require.Nil(t, tc.missingVars)
					require.Nil(t, result.missingVarNames)
				}
			}
		})
	}
}

func TestPartialEvaluation(t *testing.T) {
	compiled, err := compileCaveat(MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
		"a": types.Default.IntType,
		"b": types.Default.IntType,
	}), "a + b > 47")
	require.NoError(t, err)

	result, err := EvaluateCaveat(compiled, map[string]any{
		"a": 42,
	})
	require.NoError(t, err)
	require.False(t, result.Value())
	require.True(t, result.IsPartial())

	partialValue, err := result.PartialValue()
	require.NoError(t, err)

	astExpr, err := cel.AstToString(partialValue.ast)
	require.NoError(t, err)
	require.Equal(t, "42 + b > 47", astExpr)

	fullResult, err := EvaluateCaveat(partialValue, map[string]any{
		"b": 6,
	})
	require.NoError(t, err)
	require.True(t, fullResult.Value())
	require.False(t, fullResult.IsPartial())

	fullResult, err = EvaluateCaveat(partialValue, map[string]any{
		"b": 2,
	})
	require.NoError(t, err)
	require.False(t, fullResult.Value())
	require.False(t, fullResult.IsPartial())
}

func TestEvalWithMaxCost(t *testing.T) {
	compiled, err := compileCaveat(MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
		"a": types.Default.IntType,
		"b": types.Default.IntType,
	}), "a + b > 47")
	require.NoError(t, err)

	_, err = EvaluateCaveatWithConfig(compiled, map[string]any{
		"a": 42,
		"b": 4,
	}, &EvaluationConfig{
		MaxCost: 1,
	})
	require.Error(t, err)
	require.Equal(t, "operation cancelled: actual cost limit exceeded", err.Error())
}

func TestEvalWithNesting(t *testing.T) {
	compiled, err := compileCaveat(MustEnvForVariablesWithDefaultTypeSet(map[string]types.VariableType{
		"foo.a": types.Default.IntType,
		"foo.b": types.Default.IntType,
	}), "foo.a + foo.b > 47")
	require.NoError(t, err)

	result, err := EvaluateCaveat(compiled, map[string]any{
		"foo.a": 42,
		"foo.b": 4,
	})
	require.NoError(t, err)
	require.False(t, result.Value())
	require.False(t, result.IsPartial())
}
