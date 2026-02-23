package notenoughupdates

import (
	"math"
	"testing"
)

var prelude = []string{
	"(defun min (l r) (if (lt l r) l r))",
	"(defun max (l r) (if (lt l r) r l))",
	"(defun round-decimals (number places) (/ (round (* number (pow 10 places))) (pow 10 places)))",
	"(defun id (x) (if true x x))",
	"(defun npi (level0 maxLevel) (if (= level0 0) :COAL (if (= level0 maxLevel) :DIAMOND :EMERALD)))",
	"(defun api (level0) (if (= level0 0) :COAL_BLOCK :EMERALD_BLOCK))",
}

func mustParse(t *testing.T, input string) LispValue {
	t.Helper()

	LispParser := NewLispParser(prelude)
	result, err := LispParser.Parse(input)
	if err != nil {
		t.Fatalf("ParseLispString(%q) returned error: %v", input, err)
	}
	return result
}

func assertFloat(t *testing.T, input string, expected float64) {
	t.Helper()
	result := mustParse(t, input)
	got, ok := result.(float64)
	if !ok {
		t.Fatalf("ParseLispString(%q) = %v (%T), expected float64 %v", input, result, result, expected)
	}
	if got != expected {
		t.Errorf("ParseLispString(%q) = %v, expected %v", input, got, expected)
	}
}

func assertString(t *testing.T, input string, expected string) {
	t.Helper()
	result := mustParse(t, input)
	got, ok := result.(string)
	if !ok {
		t.Fatalf("ParseLispString(%q) = %v (%T), expected string %q", input, result, result, expected)
	}
	if got != expected {
		t.Errorf("ParseLispString(%q) = %q, expected %q", input, got, expected)
	}
}

func assertBool(t *testing.T, input string, expected bool) {
	t.Helper()
	result := mustParse(t, input)
	got, ok := result.(bool)
	if !ok {
		t.Fatalf("ParseLispString(%q) = %v (%T), expected bool %v", input, result, result, expected)
	}
	if got != expected {
		t.Errorf("ParseLispString(%q) = %v, expected %v", input, got, expected)
	}
}

func assertError(t *testing.T, input string) {
	t.Helper()

	LispParser := NewLispParser(prelude)
	_, err := LispParser.Parse(input)
	if err == nil {
		t.Errorf("ParseLispString(%q) expected error, got nil", input)
	}
}

func TestNpiFunction(t *testing.T) {
	assertString(t, "(npi 0 5)", "COAL")
	assertString(t, "(npi 5 5)", "DIAMOND")
	assertString(t, "(npi 3 5)", "EMERALD")
}

func TestApiFunction(t *testing.T) {
	assertString(t, "(api 0)", "COAL_BLOCK")
	assertString(t, "(api 1)", "EMERALD_BLOCK")
}

func TestPredefinedConstants(t *testing.T) {
	assertString(t, ":REDSTONE_BLOCK", "REDSTONE_BLOCK")
}

func TestMinFunction(t *testing.T) {
	assertFloat(t, "(min 3 5)", 3)
	assertFloat(t, "(min 10 2)", 2)
}

func TestMaxFunction(t *testing.T) {
	assertFloat(t, "(max 3 5)", 5)
	assertFloat(t, "(max 10 2)", 10)
}

func TestRoundDecimalsFunction(t *testing.T) {
	assertFloat(t, "(round-decimals 3.14159 2)", 3.14)
	assertFloat(t, "(round-decimals 2.71828 3)", 2.718)
}

func TestUnexpectedEOF(t *testing.T) {
	assertError(t, "(")
	assertError(t, "(defun")
}

func TestUnexpectedCloseParen(t *testing.T) {
	assertError(t, ")")
}

func TestIfFunction(t *testing.T) {
	assertFloat(t, "(if (lt 1 2) 30 40)", 30)
	assertFloat(t, "(if (lt 2 1) 30 40)", 40)
}

func TestPowFunction(t *testing.T) {
	assertFloat(t, "(pow (+ 3 2) 3.07)", math.Pow(5, 3.07))
}

func TestNestedExpressions(t *testing.T) {
	assertFloat(t, "(+ (* 3 0.5) 4.5)", 3*0.5+4.5)
}

func TestNestedExpressionsWithVariables(t *testing.T) {
	assertFloat(t, "(round (+ (/ 3 5) 1))", 2)
	assertFloat(t, "(round (+ (/ 5 5) 1))", 2)
	assertFloat(t, "(round (+ (/ 69 11) 6))", 12)
}

func TestIdFunction(t *testing.T) {
	assertFloat(t, "(id 3)", 3)
	assertString(t, "(id :COAL_BLOCK)", "COAL_BLOCK")
	assertFloat(t, "(id 69.514)", 69.514)
}

func TestListFunctions(t *testing.T) {
	assertFloat(t, "(list.at (list.new 0 50000 100000 200000 300000 400000 600000 750000 1000000 1250000 0) 5)", 400000)
	assertFloat(t, "(list.at (list.new 0 50000 100000 200000 300000 400000 600000 750000 1000000 1250000 0) 10)", 0)
}

func TestFloorFunction(t *testing.T) {
	assertFloat(t, "(* (floor 4) 500)", 2000)
}

func TestComplexExpression(t *testing.T) {
	assertFloat(t, "(+ 50 (/ (* (- 2 1) 350) 199))", 50+(1*350.0)/199.0)
}

func TestGtFunction(t *testing.T) {
	assertBool(t, "(gt 0 1)", false)
	assertBool(t, "(gt 2 1)", true)
}

func TestExpressionWithVariable(t *testing.T) {
	assertFloat(t, "(+ (* 10 5) 50)", 50+10*5)
	assertFloat(t, "(+ (* 0 5) 50)", 50+0*5)
	assertFloat(t, "(+ (* 1 5) 50)", 50+1*5)
}

func TestExpressionWithVariableAndIfStatement(t *testing.T) {
	assertFloat(t, "(if (lt 1 20) (+ 10 (* 5 0.5)) 30)", 10+5*0.5)
	assertFloat(t, "(if (lt 1 20) (+ 10 (* 1 0.5)) 30)", 10+1*0.5)
}
