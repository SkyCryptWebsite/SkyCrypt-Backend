package notenoughupdates

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type LispValue = interface{}

type LispFunc func(args ...LispValue) LispValue

type LispEnv map[string]LispValue

type lispExpr = interface{}

type LispParser struct {
	env LispEnv
}

func NewLispParser(stdlibDefuns []string) *LispParser {
	env := newBaseEnv()
	registerDefuns(env, stdlibDefuns)
	return &LispParser{env: env}
}

func (p *LispParser) SetEnv(key string, value LispValue) {
	p.env[key] = value
}

func (p *LispParser) Parse(input string) (LispValue, error) {
	tokens := tokenize(input)
	expr, err := parse(&tokens)
	if err != nil {
		return nil, err
	}
	return evaluate(expr, p.env)
}

func newBaseEnv() LispEnv {
	return LispEnv{
		"if": LispFunc(func(args ...LispValue) LispValue {
			cond := toBool(args[0])
			if cond {
				return args[1]
			}
			return args[2]
		}),
		"lt": LispFunc(func(args ...LispValue) LispValue {
			return toFloat(args[0]) < toFloat(args[1])
		}),
		"round": LispFunc(func(args ...LispValue) LispValue {
			return math.Round(toFloat(args[0]))
		}),
		"pow": LispFunc(func(args ...LispValue) LispValue {
			return math.Pow(toFloat(args[0]), toFloat(args[1]))
		}),
		"/": LispFunc(func(args ...LispValue) LispValue {
			return toFloat(args[0]) / toFloat(args[1])
		}),
		"*": LispFunc(func(args ...LispValue) LispValue {
			return toFloat(args[0]) * toFloat(args[1])
		}),
		"+": LispFunc(func(args ...LispValue) LispValue {
			return toFloat(args[0]) + toFloat(args[1])
		}),
		"=": LispFunc(func(args ...LispValue) LispValue {
			return lispEqual(args[0], args[1])
		}),
		"-": LispFunc(func(args ...LispValue) LispValue {
			return toFloat(args[0]) - toFloat(args[1])
		}),
		"true":              true,
		"false":             false,
		":COAL":             "COAL",
		":DIAMOND":          "DIAMOND",
		":EMERALD":          "EMERALD",
		":COAL_BLOCK":       "COAL_BLOCK",
		":EMERALD_BLOCK":    "EMERALD_BLOCK",
		":REDSTONE_BLOCK":   "REDSTONE_BLOCK",
		":MANGROVE_WOOD":    "MANGROVE_WOOD",
		":PALE_OAK_BUTTON":  "PALE_OAK_BUTTON",
		":STRIPPED_OAK_LOG": "STRIPPED_OAK_LOG",
		":PALE_OAK_SAPLING": "PALE_OAK_SAPLING",
		":OAK_LOG":          "OAK_LOG",
		":OAK_SAPLING":      "OAK_SAPLING",
		":MANGROVE_ROOTS":   "MANGROVE_ROOTS",
		"list.new": LispFunc(func(args ...LispValue) LispValue {
			return args
		}),
		"list.at": LispFunc(func(args ...LispValue) LispValue {
			list := args[0].([]LispValue)
			index := int(toFloat(args[1]))
			return list[index]
		}),
		"floor": LispFunc(func(args ...LispValue) LispValue {
			return math.Floor(toFloat(args[0]))
		}),
		"gt": LispFunc(func(args ...LispValue) LispValue {
			return toFloat(args[0]) >= toFloat(args[1])
		}),
	}
}

func registerDefuns(env LispEnv, defuns []string) {
	for _, line := range defuns {
		tokens := tokenize(line)
		expr, err := parse(&tokens)
		if err != nil {
			panic(fmt.Sprintf("failed to parse stdlib defun: %v", err))
		}
		_, err = evaluate(expr, env)
		if err != nil {
			panic(fmt.Sprintf("failed to evaluate stdlib defun: %v", err))
		}
	}
}

func tokenize(input string) []string {
	input = strings.ReplaceAll(input, "(", " ( ")
	input = strings.ReplaceAll(input, ")", " ) ")
	input = strings.TrimSpace(input)
	return strings.Fields(input)
}

// parse recursively parses tokens into a nested expression tree.
func parse(tokens *[]string) (lispExpr, error) {
	if len(*tokens) == 0 {
		return nil, fmt.Errorf("unexpected EOF")
	}

	token := (*tokens)[0]
	*tokens = (*tokens)[1:]

	if token == "(" {
		list := []lispExpr{}
		for {
			if len(*tokens) == 0 {
				return nil, fmt.Errorf("unexpected EOF")
			}
			if (*tokens)[0] == ")" {
				*tokens = (*tokens)[1:] // consume ')'
				break
			}
			val, err := parse(tokens)
			if err != nil {
				return nil, err
			}
			list = append(list, val)
		}
		return list, nil
	} else if token == ")" {
		return nil, fmt.Errorf("unexpected )")
	}

	return atom(token), nil
}

// atom converts a token string to a float64 (if numeric) or keeps it as a string.
func atom(token string) lispExpr {
	if f, err := strconv.ParseFloat(token, 64); err == nil {
		return f
	}
	return token
}

func isExpression(s string) bool {
	return strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")
}

func evaluate(x lispExpr, env LispEnv) (LispValue, error) {
	switch v := x.(type) {
	case string:
		if isExpression(v) {
			tokens := tokenize(v)
			parsed, err := parse(&tokens)
			if err != nil {
				return nil, err
			}
			return evaluate(parsed, env)
		}
		val, ok := env[v]
		if !ok {
			return nil, fmt.Errorf("undefined symbol: %s", v)
		}
		return val, nil

	case float64:
		return v, nil

	case []lispExpr:
		if len(v) == 0 {
			return nil, fmt.Errorf("empty expression")
		}

		// Check for defun special form
		if sym, ok := v[0].(string); ok && sym == "defun" {
			name := v[1].(string)
			params := v[2].([]lispExpr)
			body := v[3]

			paramNames := make([]string, len(params))
			for i, p := range params {
				paramNames[i] = p.(string)
			}

			env[name] = LispFunc(func(args ...LispValue) LispValue {
				localEnv := copyEnv(env)
				for i, param := range paramNames {
					localEnv[param] = args[i]
				}
				result, err := evaluate(body, localEnv)
				if err != nil {
					panic(fmt.Sprintf("error in defun %s: %v", name, err))
				}
				return result
			})
			return nil, nil
		}

		// Check for 'if' special form — lazy evaluation of branches
		if sym, ok := v[0].(string); ok && sym == "if" {
			condVal, err := evaluate(v[1], env)
			if err != nil {
				return nil, err
			}
			if toBool(condVal) {
				return evaluate(v[2], env)
			}
			return evaluate(v[3], env)
		}

		// Normal function call: evaluate all subexpressions
		evaluated := make([]LispValue, len(v))
		for i, exp := range v {
			val, err := evaluate(exp, env)
			if err != nil {
				return nil, err
			}
			evaluated[i] = val
		}

		proc, ok := evaluated[0].(LispFunc)
		if !ok {
			return nil, fmt.Errorf("expected a function but got %T", evaluated[0])
		}

		return proc(evaluated[1:]...), nil

	default:
		return nil, fmt.Errorf("unexpected expression type: %T", x)
	}
}

func copyEnv(env LispEnv) LispEnv {
	newEnv := make(LispEnv, len(env))
	for k, v := range env {
		newEnv[k] = v
	}
	return newEnv
}

func toFloat(v LispValue) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		panic(fmt.Sprintf("cannot convert %T to float64", v))
	}
}

func toBool(v LispValue) bool {
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val != ""
	case nil:
		return false
	default:
		return true
	}
}

func lispEqual(a, b LispValue) bool {
	switch av := a.(type) {
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
		return false
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
		return false
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
		return false
	default:
		return false
	}
}
