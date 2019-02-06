package presto

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/davecgh/go-spew/spew"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/pkg/errors"
	"github.com/segmentio/go-athena/presto/internal"
)

func ValidateAndFormatSql(sql string, params ...interface{}) (string, error) {
	err := checkSyntax(sql)
	if err != nil {
		return "", err
	}

	newSql := strings.Builder{}
	is := antlr.NewInputStream(sql)
	is2 := newUpcaseCharStream(is)
	lexer := internal.NewSqlBaseLexer(is2)

	for {
		t := lexer.NextToken()
		if t.GetTokenType() == antlr.TokenEOF {
			break
		}

		if t.GetTokenType() == internal.SqlBaseLexerPARAMETER {
			indexString := t.GetText()[1:]
			if len(indexString) == 0 {
				return "", errors.New("Missing parameter index")
			}
			index, err := strconv.ParseInt(indexString, 10, 64)
			if err != nil {
				return "", err
			}

			if index < 0 || int(index) > len(params) {
				return "", errors.Errorf("Parameter index %d out of range (%d)", index, len(params))
			}

			s, err := formatParameter(params[index-1])
			if err != nil {
				return "", err
			}
			newSql.WriteString(s)
		} else {
			newSql.WriteString(t.GetText())
		}

	}

	fmt.Println(newSql.String())

	return newSql.String(), nil
}

func formatParameter(p interface{}) (string, error) {
	switch reflect.ValueOf(p).Kind() {
	case reflect.String:
		return "'" + escapeString(p.(string), '\'', '\'') + "'", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(reflect.ValueOf(p).Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(reflect.ValueOf(p).Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(reflect.ValueOf(p).Float(), 'f', -1, 64), nil
	case reflect.Bool:
		return strconv.FormatBool(reflect.ValueOf(p).Bool()), nil
	default:
		spew.Dump(p)
		return "", errors.Errorf("Unsupported data type: %s", reflect.TypeOf(p).Name())
	}
}

func escapeString(s string, delim byte, escape byte) string {
	buf := make([]byte, 0, 3*len(s)/2)

	for width := 0; len(s) > 0; s = s[width:] {
		r := rune(s[0])
		width = 1
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRuneInString(s)
		}
		if width == 1 && r == utf8.RuneError {
			continue
		}

		if r == rune(delim) {
			buf = append(buf, escape)
			buf = append(buf, byte(r))
			continue
		}

		var runeTmp [utf8.UTFMax]byte
		if strconv.IsPrint(r) {
			n := utf8.EncodeRune(runeTmp[:], r)
			buf = append(buf, runeTmp[:n]...)
			continue
		}
	}

	return string(buf)
}

func checkSyntax(sql string) error {
	is := antlr.NewInputStream(sql)
	is2 := newUpcaseCharStream(is)
	lexer := internal.NewSqlBaseLexer(is2)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := internal.NewSqlBaseParser(stream)
	p.AddErrorListener(antlr.NewDiagnosticErrorListener(true))
	p.BuildParseTrees = true
	p.RemoveErrorListeners()
	el := &errorListener{}
	p.AddErrorListener(el)
	_ = p.SingleStatement()

	return el.err
}

type errorListener struct {
	*antlr.DefaultErrorListener

	err error
}

func (e *errorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, ex antlr.RecognitionException) {
	if e.err == nil {
		text := ""
		if token, ok := offendingSymbol.(*antlr.CommonToken); ok {
			text = token.GetText()
		}
		e.err = errors.Errorf("Syntax error at %d:%d: '%v' (%s)", line, column, text, msg)
	}
}

type upcaseCharStream struct {
	antlr.CharStream
}

func newUpcaseCharStream(s antlr.CharStream) *upcaseCharStream {
	return &upcaseCharStream{CharStream: s}
}

func (is *upcaseCharStream) LA(offset int) int {
	c := is.CharStream.LA(offset)
	if c < 0 {
		return c
	}

	return int(unicode.ToUpper(rune(c)))
}
