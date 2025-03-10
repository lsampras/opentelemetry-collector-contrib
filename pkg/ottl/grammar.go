// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
import (
	"encoding/hex"
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
)

// parsedStatement represents a parsed statement. It is the entry point into the statement DSL.
type parsedStatement struct {
	Invocation  invocation         `parser:"@@"`
	WhereClause *booleanExpression `parser:"( 'where' @@ )?"`
}

// booleanValue represents something that evaluates to a boolean --
// either an equality or inequality, explicit true or false, or
// a parenthesized subexpression.
type booleanValue struct {
	Comparison *comparison        `parser:"( @@"`
	ConstExpr  *boolean           `parser:"| @Boolean"`
	SubExpr    *booleanExpression `parser:"| '(' @@ ')' )"`
}

// opAndBooleanValue represents the right side of an AND boolean expression.
type opAndBooleanValue struct {
	Operator string        `parser:"@OpAnd"`
	Value    *booleanValue `parser:"@@"`
}

// term represents an arbitrary number of boolean values joined by AND.
type term struct {
	Left  *booleanValue        `parser:"@@"`
	Right []*opAndBooleanValue `parser:"@@*"`
}

// opOrTerm represents the right side of an OR boolean expression.
type opOrTerm struct {
	Operator string `parser:"@OpOr"`
	Term     *term  `parser:"@@"`
}

// booleanExpression represents a true/false decision expressed
// as an arbitrary number of terms separated by OR.
type booleanExpression struct {
	Left  *term       `parser:"@@"`
	Right []*opOrTerm `parser:"@@*"`
}

// compareOp is the type of a comparison operator.
type compareOp int

// These are the allowed values of a compareOp
const (
	EQ compareOp = iota
	NE
	LT
	LTE
	GTE
	GT
)

// a fast way to get from a string to a compareOp
var compareOpTable = map[string]compareOp{
	"==": EQ,
	"!=": NE,
	"<":  LT,
	"<=": LTE,
	">":  GT,
	">=": GTE,
}

// Capture is how the parser converts an operator string to a compareOp.
func (c *compareOp) Capture(values []string) error {
	op, ok := compareOpTable[values[0]]
	if !ok {
		return fmt.Errorf("'%s' is not a valid operator", values[0])
	}
	*c = op
	return nil
}

// String() for compareOp gives us more legible test results and error messages.
func (c *compareOp) String() string {
	switch *c {
	case EQ:
		return "EQ"
	case NE:
		return "NE"
	case LT:
		return "LT"
	case LTE:
		return "LTE"
	case GTE:
		return "GTE"
	case GT:
		return "GT"
	default:
		return "UNKNOWN OP!"
	}
}

// comparison represents an optional boolean condition.
type comparison struct {
	Left  value     `parser:"@@"`
	Op    compareOp `parser:"@OpComparison"`
	Right value     `parser:"@@"`
}

// invocation represents a function call.
type invocation struct {
	Function  string  `parser:"@(Uppercase | Lowercase)+"`
	Arguments []value `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
}

// value represents a part of a parsed statement which is resolved to a value of some sort. This can be a telemetry path
// expression, function call, or literal.
type value struct {
	Invocation *invocation `parser:"( @@"`
	Bytes      *byteSlice  `parser:"| @Bytes"`
	String     *string     `parser:"| @String"`
	Float      *float64    `parser:"| @Float"`
	Int        *int64      `parser:"| @Int"`
	Bool       *boolean    `parser:"| @Boolean"`
	IsNil      *isNil      `parser:"| @'nil'"`
	Enum       *EnumSymbol `parser:"| @Uppercase"`
	Path       *Path       `parser:"| @@ )"`
}

// Path represents a telemetry path expression.
type Path struct {
	Fields []Field `parser:"@@ ( '.' @@ )*"`
}

// Field is an item within a Path.
type Field struct {
	Name   string  `parser:"@Lowercase"`
	MapKey *string `parser:"( '[' @String ']' )?"`
}

// byteSlice type for capturing byte slices
type byteSlice []byte

func (b *byteSlice) Capture(values []string) error {
	rawStr := values[0][2:]
	newBytes, err := hex.DecodeString(rawStr)
	if err != nil {
		return err
	}
	*b = newBytes
	return nil
}

// boolean Type for capturing booleans, see:
// https://github.com/alecthomas/participle#capturing-boolean-value
type boolean bool

func (b *boolean) Capture(values []string) error {
	*b = values[0] == "true"
	return nil
}

type isNil bool

func (n *isNil) Capture(_ []string) error {
	*n = true
	return nil
}

type EnumSymbol string

// buildLexer constructs a SimpleLexer definition.
// Note that the ordering of these rules matters.
// It's in a separate function so it can be easily tested alone (see lexer_test.go).
func buildLexer() *lexer.StatefulDefinition {
	return lexer.MustSimple([]lexer.SimpleRule{
		{Name: `Bytes`, Pattern: `0x[a-fA-F0-9]+`},
		{Name: `Float`, Pattern: `[-+]?\d*\.\d+([eE][-+]?\d+)?`},
		{Name: `Int`, Pattern: `[-+]?\d+`},
		{Name: `String`, Pattern: `"(\\"|[^"])*"`},
		{Name: `OpOr`, Pattern: `\b(or)\b`},
		{Name: `OpAnd`, Pattern: `\b(and)\b`},
		{Name: `OpComparison`, Pattern: `==|!=|>=|<=|>|<`},
		{Name: `Boolean`, Pattern: `\b(true|false)\b`},
		{Name: `LParen`, Pattern: `\(`},
		{Name: `RParen`, Pattern: `\)`},
		{Name: `Punct`, Pattern: `[,.\[\]]`},
		{Name: `Uppercase`, Pattern: `[A-Z_][A-Z0-9_]*`},
		{Name: `Lowercase`, Pattern: `[a-z_][a-z0-9_]*`},
		{Name: "whitespace", Pattern: `\s+`},
	})
}
