package service

import (
	"fmt"
	"strings"

	"github.com/enfabrica/enkit/lib/logger"
)

const (
	OperatorAND string = "AND"
	OperatorOR string = "OR"
	OperatorNOT string = "NOT"
	OperatorEQ string = "="
	OperatorGTE string = ">="
)

type Operator struct {
	Precedence int
	NumOperands int
	RightAssociated bool
	Operands []Token
}

func getOperator(opValue string) (*Operator, error) {
	operator := Operator{NumOperands: 2}

	switch opValue {
	case OperatorOR:
		operator.Precedence = 0
	case OperatorAND:
		operator.Precedence = 1
	case OperatorEQ, OperatorGTE:
		operator.Precedence = 2
	case OperatorNOT:
		operator.Precedence = 3
		operator.RightAssociated = true
		operator.NumOperands = 1
	default:
		return nil, fmt.Errorf("Invalid operator value: %s", opValue)
	}

	return &operator, nil
}

type Token struct {
	Value string
	Operator *Operator
}

func (t Token) String() string {
	if t.Operator != nil && len(t.Operator.Operands) > 0 {
		return fmt.Sprintf("%v%v", t.Value, t.Operator.Operands)
	} else {
		return t.Value
	}
}

func (t Token) Evaluate(unit *unit) (bool, error) {
	if t.Operator == nil {
		return false, fmt.Errorf("Cannot call Evaluate on an operand: %v", t)
	}
	
	if len(t.Operator.Operands) != t.Operator.NumOperands {
		return false, fmt.Errorf("Incorrect number of operands for %v. Expected %d: %v", t.Value, t.Operator.NumOperands, t.Operator.Operands)
	}

	var boolResult bool

	if t.Operator.RightAssociated && t.Operator.NumOperands == 1 {
		// If this is a Right Associated operator, we should only need one operand
		// If the operand is, itself, an operator, we will call Evaluate on it to get it's Boolean value. Otherwise, we will 
		//   treat the operand as the name of a Unit field and will de-reference the Unit for its value
		singleOperand := t.Operator.Operands[0]
		if singleOperand.isOperator() {
			logger.Go.Debugf("Evaluating: %v", singleOperand)
			return singleOperand.Evaluate(unit)
		} else {
			logger.Go.Debugf("TODO: Access Unit field for: %v", singleOperand.Value)			
		}

		if t.Value == OperatorNOT {
			logger.Go.Debugf("TODO: Apply NOT logic to single value")
			boolResult = true  // TODO: Remove
		} else {
			return false, fmt.Errorf("Operator %v does not support right-associated behavior", t)
		}
	} else if !t.Operator.RightAssociated && t.Operator.NumOperands == 2 {		// If it's not Right Associated, we should need 2 operands. For these operands, if they are operators, we will call Evaluate on them
		//   and if they're not, we will treat the LEFT as the Unit field, same as above, and the RIGHT as a constant value that will be converted accordingly
		//   If our two operands are one operator and one operand, that's an error
		rightOperand := t.Operator.Operands[0]
		leftOperand := t.Operator.Operands[1]
		if rightOperand.isOperator() && leftOperand.isOperator() {
			logger.Go.Debugf("Evaluating: %v", leftOperand)
			leftOperand.Evaluate(unit)
			logger.Go.Debugf("Evaluating: %v", rightOperand)
			rightOperand.Evaluate(unit)
		} else if rightOperand.isOperand() && leftOperand.isOperand() {
			logger.Go.Debugf("TODO: Access Unit field for: %v", leftOperand.Value)
			logger.Go.Debugf("TODO: Convert string appropriately for: %v", rightOperand.Value)
			boolResult = true  // TODO: Remove
		} else {
			return false, fmt.Errorf("Both operands for %v must be either operators or operands, not one of each: %v", t.Value, t.Operator.Operands)
		}

		switch t.Value {
		case OperatorAND:
			logger.Go.Debugf("TODO: Apply AND logic to left and right values")
		case OperatorOR:
			logger.Go.Debugf("TODO: Apply OR logic to left and right values")
		case OperatorEQ:
			logger.Go.Debugf("TODO: Apply EQ logic to left and right values")
		case OperatorGTE:
			logger.Go.Debugf("TODO: Apply GTE logic to left and right values")
		}
	} else {
		return false, fmt.Errorf("Invalid operator type. Right-Associated: %v with %v operands", t.Operator.RightAssociated, t.Operator.NumOperands)
	}

	return boolResult, nil
}

func (t Token) isOperator() bool {
	return t.Operator != nil
}

func (t Token) isOperand() bool {
	return t.Operator == nil
}

type Query struct {
	Value string
}

func (q Query) String() string {
	return q.Value
}

func (q Query) getPostFix() ([]Token, error) {
	var outQueue []Token
	var opStack []Token
	
	tokenValues := strings.Fields(q.Value)

	for _, tokenValue := range tokenValues {
		operator, _ := getOperator(tokenValue)
		token := Token{Value: tokenValue, Operator: operator}

		if operator != nil {
			// apply order-of-operations precedence for this operator
			// i.e. while we still have operators in our stack and they are higher in precedence
			for len(opStack) > 0 {
				lastOpToken := opStack[len(opStack)-1]
				if (operator.RightAssociated && operator.Precedence < lastOpToken.Operator.Precedence) || (!operator.RightAssociated && operator.Precedence <= lastOpToken.Operator.Precedence) {
					// move the higher-precedence op to the output queue from our stack
					// logger.Go.Debugf("Moving %v from stack to outq", lastOpToken)
					outQueue = append(outQueue, lastOpToken)
					opStack = opStack[:len(opStack)-1]
					continue
				}
				break
			}
			// logger.Go.Debugf("Adding %v to opStack", token)
			opStack = append(opStack, token)
		} else {
			// not a valid operator, treat this as an operand
			outQueue = append(outQueue, token)
		}

		// logger.Go.Debugf("  OutQ: %v", outQueue)
		// logger.Go.Debugf("  Op-Stack: %v", opStack)
	}

	// move all remaining ops onto the out queue from our stack
	for len(opStack) > 0 {
		outQueue = append(outQueue, opStack[len(opStack)-1])
		opStack = opStack[:len(opStack)-1]
	}

	logger.Go.Infof("PostFix: %v", outQueue)

	return outQueue, nil
}

func (q Query) satisfiedByUnit(unit *unit) (bool, error) {
	postFixTokens, err := q.getPostFix()
	if err != nil {
		return false, fmt.Errorf("Failed to convert query to post-fix notation: %v", err)
	}

	var stack []Token

	for _, token := range postFixTokens {
		if token.Operator == nil {
			// operand, push it on the stack
			stack = append(stack, token)
		} else {
			// operator, process it accordingly
			if len(stack) < token.Operator.NumOperands {
				return false, fmt.Errorf("Not enough operands for %v operation: %v", token.Value, stack)
			}

			for i := 0; i < token.Operator.NumOperands; i++ {
				stackTop := stack[len(stack)-1]

				token.Operator.Operands = append(token.Operator.Operands, stackTop)
				stack = stack[:len(stack)-1]
			}

			stack = append(stack, token)
		}
	}

	// At this point, a properly formed expression has been reduced to a single item on the stack
	// When we evaluate that item, the logic propagates to evaluate each sub item to determine the 
	// overall result of the expression against this unit
	if len(stack) != 1 {
		return false, fmt.Errorf("Improperly formed expression. Cannot be evaluated. Please check your syntax: %v", q)
	}

	_, err = stack[0].Evaluate(unit)

	if err != nil {
		return false, fmt.Errorf("Failed to evaluate %v: %v", stack[0], err)
	}

	return true, nil
}
