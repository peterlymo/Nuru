package evaluator

import (
	"fmt"
	"math"
	"strings"

	"github.com/AvicennaJr/Nuru/ast"
	"github.com/AvicennaJr/Nuru/object"
)

var (
	NULL     = &object.Null{}
	TRUE     = &object.Boolean{Value: true}
	FALSE    = &object.Boolean{Value: false}
	BREAK    = &object.Break{}
	CONTINUE = &object.Continue{}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node, env)

	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}

	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right, node.Token.Line)

	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right, node.Token.Line)
	case *ast.PostfixExpression:
		return evalPostfixExpression(env, node.Operator, node)

	case *ast.BlockStatement:
		return evalBlockStatement(node, env)

	case *ast.IfExpression:
		return evalIfExpression(node, env)

	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}

	case *ast.LetStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}

		env.Set(node.Name.Value, val)

	case *ast.Identifier:
		return evalIdentifier(node, env)

	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Env: env, Body: body}

	case *ast.CallExpression:
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args, node.Token.Line)
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}

	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index, node.Token.Line)
	case *ast.DictLiteral:
		return evalDictLiteral(node, env)
	case *ast.WhileExpression:
		return evalWhileExpression(node, env)
	case *ast.Break:
		return evalBreak(node)
	case *ast.Continue:
		return evalContinue(node)
	case *ast.SwitchExpression:
		return evalSwitchStatement(node, env)
	case *ast.Null:
		return NULL
	// case *ast.For:
	// 	return evalForExpression(node, env)
	case *ast.ForIn:
		return evalForInExpression(node, env, node.Token.Line)
	case *ast.AssignmentExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}

		value := Eval(node.Value, env)
		if isError(value) {
			return value
		}

		// This is an easy way to assign operators like +=, -= etc
		// I'm surprised it work at the first try lol
		// basically separate the += to + and =, take the + only and
		// then perform the operation as normal
		op := node.Token.Literal
		if len(op) >= 2 {
			op = op[:len(op)-1]
			value = evalInfixExpression(op, left, value, node.Token.Line)
			if isError(value) {
				return value
			}
		}

		if ident, ok := node.Left.(*ast.Identifier); ok {
			env.Set(ident.Value, value)
		} else if ie, ok := node.Left.(*ast.IndexExpression); ok {
			obj := Eval(ie.Left, env)
			if isError(obj) {
				return obj
			}

			if array, ok := obj.(*object.Array); ok {
				index := Eval(ie.Index, env)
				if isError(index) {
					return index
				}
				if idx, ok := index.(*object.Integer); ok {
					if int(idx.Value) > len(array.Elements) {
						return newError("Index imezidi idadi ya elements")
					}
					array.Elements[idx.Value] = value
				} else {
					return newError("Hauwezi kufanya opereshen hii na %#v", index)
				}
			} else if hash, ok := obj.(*object.Dict); ok {
				key := Eval(ie.Index, env)
				if isError(key) {
					return key
				}
				if hashKey, ok := key.(object.Hashable); ok {
					hashed := hashKey.HashKey()
					hash.Pairs[hashed] = object.DictPair{Key: key, Value: value}
				} else {
					return newError("Hauwezi kufanya opereshen hii na %T", key)
				}
			} else {
				return newError("%T haifanyi operation hii", obj)
			}
		} else {
			return newError("Tumia neno kama variable, sio %T", left)
		}

	}

	return nil
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statment := range program.Statements {
		result = Eval(statment, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right object.Object, line int) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right, line)
	case "+":
		return evalPlusPrefixOperatorExpression(right, line)
	default:
		return newError("Mstari %d: Operesheni haieleweki: %s%s", line, operator, right.Type())
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right object.Object, line int) object.Object {
	switch obj := right.(type) {

	case *object.Integer:
		return &object.Integer{Value: -obj.Value}

	case *object.Float:
		return &object.Float{Value: -obj.Value}

	default:
		return newError("Mstari %d: Operesheni Haielweki: -%s", line, right.Type())
	}
}
func evalPlusPrefixOperatorExpression(right object.Object, line int) object.Object {
	switch obj := right.(type) {

	case *object.Integer:
		return &object.Integer{Value: obj.Value}

	case *object.Float:
		return &object.Float{Value: obj.Value}

	default:
		return newError("Mstari %d: Operesheni Haielweki: -%s", line, right.Type())
	}
}
func evalInfixExpression(operator string, left, right object.Object, line int) object.Object {
	if left == nil {
		return newError("Mstari %d: Umekosea hapa", line)
	}
	switch {
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right, line)

	case operator == "+" && left.Type() == object.DICT_OBJ && right.Type() == object.DICT_OBJ:
		leftVal := left.(*object.Dict).Pairs
		rightVal := right.(*object.Dict).Pairs
		pairs := make(map[object.HashKey]object.DictPair)
		for k, v := range leftVal {
			pairs[k] = v
		}
		for k, v := range rightVal {
			pairs[k] = v
		}
		return &object.Dict{Pairs: pairs}

	case operator == "+" && left.Type() == object.ARRAY_OBJ && right.Type() == object.ARRAY_OBJ:
		leftVal := left.(*object.Array).Elements
		rightVal := right.(*object.Array).Elements
		elements := make([]object.Object, len(leftVal)+len(rightVal))
		elements = append(leftVal, rightVal...)
		return &object.Array{Elements: elements}

	case operator == "*" && left.Type() == object.ARRAY_OBJ && right.Type() == object.INTEGER_OBJ:
		leftVal := left.(*object.Array).Elements
		rightVal := int(right.(*object.Integer).Value)
		elements := leftVal
		for i := rightVal; i > 1; i-- {
			elements = append(elements, leftVal...)
		}
		return &object.Array{Elements: elements}

	case operator == "*" && left.Type() == object.INTEGER_OBJ && right.Type() == object.ARRAY_OBJ:
		leftVal := int(left.(*object.Integer).Value)
		rightVal := right.(*object.Array).Elements
		elements := rightVal
		for i := leftVal; i > 1; i-- {
			elements = append(elements, rightVal...)
		}
		return &object.Array{Elements: elements}

	case operator == "*" && left.Type() == object.STRING_OBJ && right.Type() == object.INTEGER_OBJ:
		leftVal := left.(*object.String).Value
		rightVal := right.(*object.Integer).Value
		return &object.String{Value: strings.Repeat(leftVal, int(rightVal))}

	case operator == "*" && left.Type() == object.INTEGER_OBJ && right.Type() == object.STRING_OBJ:
		leftVal := left.(*object.Integer).Value
		rightVal := right.(*object.String).Value
		return &object.String{Value: strings.Repeat(rightVal, int(leftVal))}

	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right, line)

	case left.Type() == object.FLOAT_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalFloatInfixExpression(operator, left, right, line)

	case left.Type() == object.INTEGER_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalFloatIntegerInfixExpression(operator, left, right, line)

	case left.Type() == object.FLOAT_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalFloatIntegerInfixExpression(operator, left, right, line)

	case operator == "ktk":
		return evalInExpression(left, right, line)

	case operator == "==":
		return nativeBoolToBooleanObject(left == right)

	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	case left.Type() == object.BOOLEAN_OBJ && right.Type() == object.BOOLEAN_OBJ:
		return evalBooleanInfixExpression(operator, left, right, line)

	case left.Type() != right.Type():
		return newError("Mstari %d: Aina Hazilingani: %s %s %s",
			line, left.Type(), operator, right.Type())

	default:
		return newError("Mstari %d: Operesheni Haielweki: %s %s %s",
			line, left.Type(), operator, right.Type())
	}
}

func evalIntegerInfixExpression(operator string, left, right object.Object, line int) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "**":
		return &object.Integer{Value: int64(math.Pow(float64(leftVal), float64(rightVal)))}
	case "/":
		x := float64(leftVal) / float64(rightVal)
		if math.Mod(x, 1) == 0 {
			return &object.Integer{Value: int64(x)}
		} else {
			return &object.Float{Value: x}
		}
	case "%":
		return &object.Integer{Value: leftVal % rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("Mstari %d: Operesheni Haielweki: %s %s %s",
			line, left.Type(), operator, right.Type())
	}
}

func evalFloatInfixExpression(operator string, left, right object.Object, line int) object.Object {
	leftVal := left.(*object.Float).Value
	rightVal := right.(*object.Float).Value

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "**":
		return &object.Float{Value: math.Pow(float64(leftVal), float64(rightVal))}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("Mstari %d: Operesheni Haielweki: %s %s %s",
			line, left.Type(), operator, right.Type())
	}
}

func evalFloatIntegerInfixExpression(operator string, left, right object.Object, line int) object.Object {
	var leftVal, rightVal float64
	if left.Type() == object.FLOAT_OBJ {
		leftVal = left.(*object.Float).Value
		rightVal = float64(right.(*object.Integer).Value)
	} else {
		leftVal = float64(left.(*object.Integer).Value)
		rightVal = right.(*object.Float).Value
	}

	var val float64
	switch operator {
	case "+":
		val = leftVal + rightVal
	case "-":
		val = leftVal - rightVal
	case "*":
		val = leftVal * rightVal
	case "**":
		val = math.Pow(float64(leftVal), float64(rightVal))
	case "/":
		val = leftVal / rightVal
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("Mstari %d: Operesheni Haielweki: %s %s %s",
			line, left.Type(), operator, right.Type())
	}

	if math.Mod(val, 1) == 0 {
		return &object.Integer{Value: int64(val)}
	} else {
		return &object.Float{Value: val}
	}
}

func evalBooleanInfixExpression(operator string, left, right object.Object, line int) object.Object {
	leftVal := left.(*object.Boolean).Value
	rightVal := right.(*object.Boolean).Value

	switch operator {
	case "&&":
		return nativeBoolToBooleanObject(leftVal && rightVal)
	case "||":
		return nativeBoolToBooleanObject(leftVal || rightVal)
	default:
		return newError("Mstari %d: Operesheni Haielweki: %s %s %s", line, left.Type(), operator, right.Type())
	}
}

func evalPostfixExpression(env *object.Environment, operator string, node *ast.PostfixExpression) object.Object {
	val, ok := env.Get(node.Token.Literal)
	if !ok {
		return newError("Tumia KITAMBULISHI CHA NAMBA AU DESIMALI, sio %s", node.Token.Type)
	}
	switch operator {
	case "++":
		switch arg := val.(type) {
		case *object.Integer:
			v := arg.Value + 1
			return env.Set(node.Token.Literal, &object.Integer{Value: v})
		case *object.Float:
			v := arg.Value + 1
			return env.Set(node.Token.Literal, &object.Float{Value: v})
		default:
			return newError("Mstari %d: %s sio kitambulishi cha namba. Tumia '++' na kitambulishi cha namba au desimali.\nMfano:\tfanya i = 2; i++", node.Token.Line, node.Token.Literal)

		}
	case "--":
		switch arg := val.(type) {
		case *object.Integer:
			v := arg.Value - 1
			return env.Set(node.Token.Literal, &object.Integer{Value: v})
		case *object.Float:
			v := arg.Value - 1
			return env.Set(node.Token.Literal, &object.Float{Value: v})
		default:
			return newError("Mstari %d: %s sio kitambulishi cha namba. Tumia '--' na kitambulishi cha namba au desimali.\nMfano:\tfanya i = 2; i++", node.Token.Line, node.Token.Literal)
		}
	default:
		return newError("Haifahamiki: %s", operator)
	}
}

func evalIfExpression(ie *ast.IfExpression, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)

	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object

	for _, statment := range block.Statements {
		result = Eval(statment, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ || rt == object.CONTINUE_OBJ || rt == object.BREAK_OBJ {
				return result
			}
		}
	}

	return result
}

func newError(format string, a ...interface{}) *object.Error {
	format = fmt.Sprintf("\x1b[%dm%s\x1b[0m", 31, format)
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}

	return false
}

func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}

	return newError("Mstari %d: Neno Halifahamiki: %s", node.Token.Line, node.Value)
}

func evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}

		result = append(result, evaluated)
	}

	return result
}

func applyFunction(fn object.Object, args []object.Object, line int) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		extendedEnv := extendedFunctionEnv(fn, args)
		evaluated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)
	case *object.Builtin:
		if result := fn.Fn(args...); result != nil {
			return result
		}
		return NULL
	default:
		return newError("Mstari %d: Hii sio function: %s", line, fn.Type())
	}

}

func extendedFunctionEnv(fn *object.Function, args []object.Object) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		if paramIdx < len(args) {
			env.Set(param.Value, args[paramIdx])
		}
	}
	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}

	return obj
}

func evalStringInfixExpression(operator string, left, right object.Object, line int) object.Object {

	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	switch operator {
	case "+":
		return &object.String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("Mstari %d: Operesheni Haielweki: %s %s %s", line, left.Type(), operator, right.Type())
	}
}

func evalIndexExpression(left, index object.Object, line int) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.ARRAY_OBJ && index.Type() != object.INTEGER_OBJ:
		return newError("Mstari %d: Tafadhali tumia number, sio: %s", line, index.Type())
	case left.Type() == object.DICT_OBJ:
		return evalDictIndexExpression(left, index, line)
	default:
		return newError("Mstari %d: Operesheni hii haiwezekani kwa: %s", line, left.Type())
	}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx]
}

func evalDictLiteral(node *ast.DictLiteral, env *object.Environment) object.Object {
	pairs := make(map[object.HashKey]object.DictPair)

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return newError("Mstari %d: Hashing imeshindikana: %s", node.Token.Line, key.Type())
		}

		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}

		hashed := hashKey.HashKey()
		pairs[hashed] = object.DictPair{Key: key, Value: value}
	}

	return &object.Dict{Pairs: pairs}
}

func evalDictIndexExpression(dict, index object.Object, line int) object.Object {
	dictObject := dict.(*object.Dict)

	key, ok := index.(object.Hashable)
	if !ok {
		return newError("Mstari %d: Samahani, %s haitumiki kama key", line, index.Type())
	}

	pair, ok := dictObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}

	return pair.Value
}

func evalWhileExpression(we *ast.WhileExpression, env *object.Environment) object.Object {
	condition := Eval(we.Condition, env)
	if isError(condition) {
		return condition
	}
	if isTruthy(condition) {
		evaluated := Eval(we.Consequence, env)
		if isError(evaluated) {
			return evaluated
		}
		if evaluated != nil && evaluated.Type() == object.BREAK_OBJ {
			return evaluated
		}
		evalWhileExpression(we, env)
	}
	return NULL
}

func evalBreak(node *ast.Break) object.Object {
	return BREAK
}

func evalContinue(node *ast.Continue) object.Object {
	return CONTINUE
}

func evalInExpression(left, right object.Object, line int) object.Object {
	switch right.(type) {
	case *object.String:
		return evalInStringExpression(left, right)
	case *object.Array:
		return evalInArrayExpression(left, right)
	case *object.Dict:
		return evalInDictExpression(left, right, line)
	default:
		return FALSE
	}
}

func evalInStringExpression(left, right object.Object) object.Object {
	if left.Type() != object.STRING_OBJ {
		return FALSE
	}
	leftVal := left.(*object.String)
	rightVal := right.(*object.String)
	found := strings.Contains(rightVal.Value, leftVal.Value)
	return nativeBoolToBooleanObject(found)
}

func evalInDictExpression(left, right object.Object, line int) object.Object {
	leftVal, ok := left.(object.Hashable)
	if !ok {
		return newError("Huwezi kutumia kama 'key': %s", left.Type())
	}
	key := leftVal.HashKey()
	rightVal := right.(*object.Dict).Pairs
	_, ok = rightVal[key]
	return nativeBoolToBooleanObject(ok)
}

func evalInArrayExpression(left, right object.Object) object.Object {
	rightVal := right.(*object.Array)
	switch leftVal := left.(type) {
	case *object.Null:
		for _, v := range rightVal.Elements {
			if v.Type() == object.NULL_OBJ {
				return TRUE
			}
		}
	case *object.String:
		for _, v := range rightVal.Elements {
			if v.Type() == object.STRING_OBJ {
				elem := v.(*object.String)
				if elem.Value == leftVal.Value {
					return TRUE
				}
			}
		}
	case *object.Integer:
		for _, v := range rightVal.Elements {
			if v.Type() == object.INTEGER_OBJ {
				elem := v.(*object.Integer)
				if elem.Value == leftVal.Value {
					return TRUE
				}
			}
		}
	case *object.Float:
		for _, v := range rightVal.Elements {
			if v.Type() == object.FLOAT_OBJ {
				elem := v.(*object.Float)
				if elem.Value == leftVal.Value {
					return TRUE
				}
			}
		}
	}
	return FALSE
}

// func evalForExpression(fe *ast.For, env *object.Environment) object.Object {
// 	obj, ok := env.Get(fe.Identifier)
// 	defer func() { // stay safe and not reassign an existing variable
// 		if ok {
// 			env.Set(fe.Identifier, obj)
// 		}
// 	}()
// 	val := Eval(fe.StarterValue, env)
// 	if isError(val) {
// 		return val
// 	}

// 	env.Set(fe.StarterName.Value, val)

// 	// err := Eval(fe.Starter, env)
// 	// if isError(err) {
// 	// 	return err
// 	// }
// 	for {
// 		evaluated := Eval(fe.Condition, env)
// 		if isError(evaluated) {
// 			return evaluated
// 		}
// 		if !isTruthy(evaluated) {
// 			break
// 		}
// 		res := Eval(fe.Block, env)
// 		if isError(res) {
// 			return res
// 		}
// 		if res.Type() == object.BREAK_OBJ {
// 			break
// 		}
// 		if res.Type() == object.CONTINUE_OBJ {
// 			err := Eval(fe.Closer, env)
// 			if isError(err) {
// 				return err
// 			}
// 			continue
// 		}
// 		if res.Type() == object.RETURN_VALUE_OBJ {
// 			return res
// 		}
// 		err := Eval(fe.Closer, env)
// 		if isError(err) {
// 			return err
// 		}
// 	}
// 	return NULL
// }

func evalForInExpression(fie *ast.ForIn, env *object.Environment, line int) object.Object {
	iterable := Eval(fie.Iterable, env)
	existingKeyIdentifier, okk := env.Get(fie.Key) // again, stay safe
	existingValueIdentifier, okv := env.Get(fie.Value)
	defer func() { // restore them later on
		if okk {
			env.Set(fie.Key, existingKeyIdentifier)
		}
		if okv {
			env.Set(fie.Value, existingValueIdentifier)
		}
	}()
	switch i := iterable.(type) {
	case object.Iterable:
		defer func() {
			i.Reset()
		}()
		return loopIterable(i.Next, env, fie)
	default:
		return newError("Mstari %d: Huwezi kufanya operesheni hii na %s", line, i.Type())
	}
}

func loopIterable(next func() (object.Object, object.Object), env *object.Environment, fi *ast.ForIn) object.Object {
	k, v := next()
	for k != nil && v != nil {
		env.Set(fi.Key, k)
		env.Set(fi.Value, v)
		res := Eval(fi.Block, env)
		if isError(res) {
			return res
		}
		if res != nil {
			if res.Type() == object.BREAK_OBJ {
				break
			}
			if res.Type() == object.CONTINUE_OBJ {
				k, v = next()
				continue
			}
			if res.Type() == object.RETURN_VALUE_OBJ {
				return res
			}
		}
		k, v = next()
	}
	return NULL
}

func evalSwitchStatement(se *ast.SwitchExpression, env *object.Environment) object.Object {
	obj := Eval(se.Value, env)
	for _, opt := range se.Choices {

		if opt.Default {
			continue
		}
		for _, val := range opt.Expr {
			out := Eval(val, env)
			if obj.Type() == out.Type() && obj.Inspect() == out.Inspect() {
				blockOut := evalBlockStatement(opt.Block, env)
				return blockOut
			}
		}
	}
	for _, opt := range se.Choices {
		if opt.Default {
			out := evalBlockStatement(opt.Block, env)
			return out
		}
	}
	return nil
}
