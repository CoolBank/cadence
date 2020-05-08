/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package parser2

import (
	"fmt"
	"math/big"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/parser2/lexer"
)

const lowestBindingPower = 0

type infixFunc func(left, right ast.Expression) ast.Expression
type prefixFunc func(right ast.Expression, tokenRange ast.Range) ast.Expression
type nullDenotationFunc func(parser *parser, token lexer.Token) ast.Expression

type literal struct {
	tokenType      lexer.TokenType
	nullDenotation nullDenotationFunc
}

type infix struct {
	tokenType        lexer.TokenType
	leftBindingPower int
	rightAssociative bool
	leftDenotation   infixFunc
}

type binary struct {
	tokenType        lexer.TokenType
	leftBindingPower int
	rightAssociative bool
	operation        ast.Operation
}

type prefix struct {
	tokenType      lexer.TokenType
	bindingPower   int
	nullDenotation prefixFunc
}

type unary struct {
	tokenType    lexer.TokenType
	bindingPower int
	operation    ast.Operation
}

var nullDenotations = map[lexer.TokenType]nullDenotationFunc{}

type leftDenotationFunc func(parser *parser, left ast.Expression) ast.Expression

var leftBindingPowers = map[lexer.TokenType]int{}
var leftDenotations = map[lexer.TokenType]leftDenotationFunc{}

func define(def interface{}) {
	switch def := def.(type) {
	case infix:
		tokenType := def.tokenType

		setLeftBindingPower(tokenType, def.leftBindingPower)

		rightBindingPower := def.leftBindingPower
		if def.rightAssociative {
			rightBindingPower -= 1
		}

		setLeftDenotation(
			tokenType,
			func(parser *parser, left ast.Expression) ast.Expression {
				right := parseExpression(parser, rightBindingPower)
				return def.leftDenotation(left, right)
			},
		)

	case binary:
		define(infix{
			tokenType:        def.tokenType,
			leftBindingPower: def.leftBindingPower,
			rightAssociative: def.rightAssociative,
			leftDenotation: func(left, right ast.Expression) ast.Expression {
				return &ast.BinaryExpression{
					Operation: def.operation,
					Left:      left,
					Right:     right,
				}
			},
		})

	case literal:
		tokenType := def.tokenType
		setNullDenotation(tokenType, def.nullDenotation)
		setLeftBindingPower(tokenType, lowestBindingPower)

	case prefix:
		tokenType := def.tokenType
		setLeftBindingPower(tokenType, lowestBindingPower)
		setNullDenotation(
			tokenType,
			func(parser *parser, token lexer.Token) ast.Expression {
				right := parseExpression(parser, def.bindingPower)
				return def.nullDenotation(right, token.Range)
			},
		)

	case unary:
		define(prefix{
			tokenType:    def.tokenType,
			bindingPower: def.bindingPower,
			nullDenotation: func(right ast.Expression, tokenRange ast.Range) ast.Expression {
				return &ast.UnaryExpression{
					Operation:  def.operation,
					Expression: right,
					StartPos:   tokenRange.StartPos,
				}
			},
		})

	default:
		panic(errors.NewUnreachableError())
	}
}

func setNullDenotation(tokenType lexer.TokenType, nullDenotation nullDenotationFunc) {
	current := nullDenotations[tokenType]
	if current != nil {
		panic(fmt.Errorf(
			"null denotation for token type %s exists",
			tokenType,
		))
	}
	nullDenotations[tokenType] = nullDenotation
}

func setLeftBindingPower(tokenType lexer.TokenType, power int) {
	current := leftBindingPowers[tokenType]
	if current > power {
		return
	}
	leftBindingPowers[tokenType] = power
}

func setLeftDenotation(tokenType lexer.TokenType, leftDenotation leftDenotationFunc) {
	current := leftDenotations[tokenType]
	if current != nil {
		panic(fmt.Errorf(
			"left denotation for token type %s exists",
			tokenType,
		))
	}
	leftDenotations[tokenType] = leftDenotation
}

func init() {

	define(binary{
		tokenType:        lexer.TokenLess,
		leftBindingPower: 60,
		operation:        ast.OperationLess,
	})

	define(binary{
		tokenType:        lexer.TokenGreater,
		leftBindingPower: 60,
		operation:        ast.OperationGreater,
	})

	define(binary{
		tokenType:        lexer.TokenNilCoalesce,
		leftBindingPower: 100,
		operation:        ast.OperationNilCoalesce,
		rightAssociative: true,
	})

	define(binary{
		tokenType:        lexer.TokenPlus,
		leftBindingPower: 110,
		operation:        ast.OperationPlus,
	})

	define(binary{
		tokenType:        lexer.TokenMinus,
		leftBindingPower: 110,
		operation:        ast.OperationMinus,
	})

	define(binary{
		tokenType:        lexer.TokenStar,
		leftBindingPower: 120,
		operation:        ast.OperationMul,
	})

	define(binary{
		tokenType:        lexer.TokenSlash,
		leftBindingPower: 120,
		operation:        ast.OperationDiv,
	})

	define(literal{
		tokenType: lexer.TokenNumber,
		nullDenotation: func(_ *parser, token lexer.Token) ast.Expression {
			value, _ := new(big.Int).SetString(token.Value.(string), 10)
			return &ast.IntegerExpression{
				Value: value,
				Base:  10,
				Range: token.Range,
			}
		},
	})

	define(literal{
		tokenType: lexer.TokenIdentifier,
		nullDenotation: func(_ *parser, token lexer.Token) ast.Expression {
			switch token.Value {
			case "true":
				return &ast.BoolExpression{
					Value: true,
					Range: token.Range,
				}

			case "false":
				return &ast.BoolExpression{
					Value: false,
					Range: token.Range,
				}

			default:
				return &ast.IdentifierExpression{
					Identifier: tokenToIdentifier(token),
				}
			}
		},
	})

	define(unary{
		tokenType:    lexer.TokenMinus,
		bindingPower: 130,
		operation:    ast.OperationMinus,
	})

	define(unary{
		tokenType:    lexer.TokenPlus,
		bindingPower: 130,
		operation:    ast.OperationPlus,
	})

	define(unary{
		tokenType:    lexer.TokenLeftArrow,
		bindingPower: 130,
		operation:    ast.OperationMove,
	})

	defineNestedExpression()
	defineArrayExpression()
	defineDictionaryExpression()
	definePathExpression()
	defineConditionalExpression()

	leftBindingPowers[lexer.TokenComma] = lowestBindingPower

	leftBindingPowers[lexer.TokenColon] = lowestBindingPower

	leftBindingPowers[lexer.TokenEOF] = lowestBindingPower
}

func defineNestedExpression() {
	leftBindingPowers[lexer.TokenParenOpen] = 150
	leftBindingPowers[lexer.TokenParenClose] = lowestBindingPower
	nullDenotations[lexer.TokenParenOpen] = func(p *parser, token lexer.Token) ast.Expression {
		expression := parseExpression(p, lowestBindingPower)
		p.mustOne(lexer.TokenParenClose)
		return expression
	}
}

func defineArrayExpression() {
	leftBindingPowers[lexer.TokenBracketOpen] = 150
	leftBindingPowers[lexer.TokenBracketClose] = lowestBindingPower
	nullDenotations[lexer.TokenBracketOpen] = func(p *parser, startToken lexer.Token) ast.Expression {
		var values []ast.Expression
		for p.current.Type != lexer.TokenBracketClose {
			value := parseExpression(p, lowestBindingPower)
			values = append(values, value)
			if p.current.Type != lexer.TokenComma {
				break
			}
			p.mustOne(lexer.TokenComma)
		}
		endToken := p.mustOne(lexer.TokenBracketClose)
		return &ast.ArrayExpression{
			Values: values,
			Range: ast.Range{
				StartPos: startToken.Range.StartPos,
				EndPos:   endToken.Range.EndPos,
			},
		}
	}
}

func defineDictionaryExpression() {
	leftBindingPowers[lexer.TokenBraceOpen] = 150
	leftBindingPowers[lexer.TokenBraceClose] = lowestBindingPower
	nullDenotations[lexer.TokenBraceOpen] = func(p *parser, startToken lexer.Token) ast.Expression {
		var entries []ast.Entry
		for p.current.Type != lexer.TokenBraceClose {
			key := parseExpression(p, lowestBindingPower)
			p.mustOne(lexer.TokenColon)
			value := parseExpression(p, lowestBindingPower)
			entries = append(entries, ast.Entry{
				Key:   key,
				Value: value,
			})
			if p.current.Type != lexer.TokenComma {
				break
			}
			p.mustOne(lexer.TokenComma)
		}
		endToken := p.mustOne(lexer.TokenBraceClose)
		return &ast.DictionaryExpression{
			Entries: entries,
			Range: ast.Range{
				StartPos: startToken.Range.StartPos,
				EndPos:   endToken.Range.EndPos,
			},
		}
	}
}

func defineConditionalExpression() {
	leftBindingPowers[lexer.TokenQuestionMark] = 20
	leftDenotations[lexer.TokenQuestionMark] = func(p *parser, left ast.Expression) ast.Expression {
		testExpression := left
		thenExpression := parseExpression(p, lowestBindingPower)
		p.mustOne(lexer.TokenColon)
		elseExpression := parseExpression(p, lowestBindingPower)
		return &ast.ConditionalExpression{
			Test: testExpression,
			Then: thenExpression,
			Else: elseExpression,
		}
	}
}

func definePathExpression() {
	leftBindingPowers[lexer.TokenSlash] = 150
	nullDenotations[lexer.TokenSlash] = func(p *parser, token lexer.Token) ast.Expression {
		domain := mustIdentifier(p)
		p.mustOne(lexer.TokenSlash)
		identifier := mustIdentifier(p)
		return &ast.PathExpression{
			Domain:     domain,
			Identifier: identifier,
			StartPos:   token.Range.StartPos,
		}
	}
}

func mustIdentifier(p *parser) ast.Identifier {
	identifier := p.mustOne(lexer.TokenIdentifier)
	return tokenToIdentifier(identifier)
}

func tokenToIdentifier(identifier lexer.Token) ast.Identifier {
	return ast.Identifier{
		Identifier: identifier.Value.(string),
		Pos:        identifier.Range.StartPos,
	}
}

func parseExpression(p *parser, rightBindingPower int) ast.Expression {
	p.skipZeroOrOne(lexer.TokenSpace)
	t := p.current
	p.next()

	left := applyNullDenotation(p, t)
	p.skipZeroOrOne(lexer.TokenSpace)

	for rightBindingPower < leftBindingPower(p.current.Type) {
		t = p.current
		p.next()
		p.skipZeroOrOne(lexer.TokenSpace)

		left = applyLeftDenotation(p, t.Type, left)
	}

	return left
}

func applyNullDenotation(p *parser, token lexer.Token) ast.Expression {
	tokenType := token.Type
	nullDenotation, ok := nullDenotations[tokenType]
	if !ok {
		panic(fmt.Errorf("missing null denotation for token type: %v", tokenType))
	}
	return nullDenotation(p, token)
}

func leftBindingPower(tokenType lexer.TokenType) int {
	result, ok := leftBindingPowers[tokenType]
	if !ok {
		panic(fmt.Errorf("missing left binding power for token type: %v", tokenType))
	}
	return result
}

func applyLeftDenotation(p *parser, tokenType lexer.TokenType, left ast.Expression) ast.Expression {
	leftDenotation, ok := leftDenotations[tokenType]
	if !ok {
		panic(fmt.Errorf("missing left denotation for token type: %v", tokenType))
	}
	return leftDenotation(p, left)
}
