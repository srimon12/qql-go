package lexer

type Token struct {
	Kind  TokenKind
	Value string
	Pos   int
}

func (t Token) String() string {
	return t.Kind.String() + "(" + t.Value + ")"
}
