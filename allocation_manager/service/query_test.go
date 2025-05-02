package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	SimpleExpression string = "numGpus = 2 AND ram >= 8M OR numGpus = 4 AND ram >= 4M AND NOT broken"
	NOTMissingOperandsExpression string = "NOT"
)

func TestSimplePostFix(t *testing.T) {
	testQuery := Query{Value: SimpleExpression}

	assert.Equal(t, SimpleExpression, testQuery.Value, "Query does not contain correct expression")

	postfixTokens, err := testQuery.getPostFix()

	assert.Equalf(t, nil, err, "getting postfix tokens returned error: %v", err)
	assert.Equalf(t, 18, len(postfixTokens), "incorrect number of tokens parsed")

	lastToken := postfixTokens[len(postfixTokens)-1]
	assert.NotNilf(t, lastToken.Operator, "last token should be an operator, but isn't: %v", lastToken)
	assert.Equalf(t, "OR", lastToken.Value, "last token should be an OR, but isn't: %v", lastToken)
}

func TestMissingOperandsNOT(t *testing.T) {
	testQuery := Query{Value: NOTMissingOperandsExpression}

	success, err := testQuery.satisfiedByUnit(nil)

	assert.Falsef(t, success, "satisifedByUnit should have failed with a missing NOT operand")
	assert.NotNilf(t, err, "satisifedByUnit should have returned error with a missing NOT operand")
}

func TestSatisfies(t *testing.T) {
	testQuery := Query{Value: SimpleExpression}

	_, err := testQuery.satisfiedByUnit(nil)
	assert.Nilf(t, err, "satisfiedByUnit failed: %v", err)
}
