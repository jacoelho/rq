package expr

import "strconv"

type node interface{}

type literalNode struct {
	value any
}

type identifierNode struct {
	name string
}

type unaryNode struct {
	op    tokenType
	right node
}

type binaryNode struct {
	op    tokenType
	left  node
	right node
}

type parserState struct {
	tokens []token
	pos    int
}

func parse(input string) (node, error) {
	tokens, err := lex(input)
	if err != nil {
		return nil, err
	}

	state := parserState{tokens: tokens}
	if state.current().typ == tokenEOF {
		return nil, expressionError("expression is empty")
	}

	root, err := state.parseExpression()
	if err != nil {
		return nil, err
	}

	if token := state.current(); token.typ != tokenEOF {
		return nil, expressionError("unexpected token at position %d", token.pos)
	}

	return root, nil
}

func (p *parserState) parseExpression() (node, error) {
	return p.parseOr()
}

func (p *parserState) parseOr() (node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.current().typ == tokenOr {
		op := p.advance().typ
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}

	return left, nil
}

func (p *parserState) parseAnd() (node, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}

	for p.current().typ == tokenAnd {
		op := p.advance().typ
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}

	return left, nil
}

func (p *parserState) parseEquality() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		typ := p.current().typ
		if typ != tokenEqual && typ != tokenNotEqual {
			break
		}

		op := p.advance().typ
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}

	return left, nil
}

func (p *parserState) parseUnary() (node, error) {
	if p.current().typ == tokenNot {
		op := p.advance().typ
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryNode{op: op, right: right}, nil
	}

	return p.parsePrimary()
}

func (p *parserState) parsePrimary() (node, error) {
	tok := p.current()
	switch tok.typ {
	case tokenIdentifier:
		p.advance()
		return identifierNode{name: tok.literal}, nil
	case tokenNumber:
		p.advance()
		value, err := strconv.ParseFloat(tok.literal, 64)
		if err != nil {
			return nil, expressionError("invalid number literal %q at position %d", tok.literal, tok.pos)
		}
		return literalNode{value: value}, nil
	case tokenString:
		p.advance()
		return literalNode{value: tok.literal}, nil
	case tokenTrue:
		p.advance()
		return literalNode{value: true}, nil
	case tokenFalse:
		p.advance()
		return literalNode{value: false}, nil
	case tokenNull:
		p.advance()
		return literalNode{value: nil}, nil
	case tokenLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.current().typ != tokenRParen {
			return nil, expressionError("missing closing ')' at position %d", p.current().pos)
		}
		p.advance()
		return expr, nil
	default:
		return nil, expressionError("unexpected token at position %d", tok.pos)
	}
}

func (p *parserState) current() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokenEOF, pos: len(p.tokens)}
	}
	return p.tokens[p.pos]
}

func (p *parserState) advance() token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}
