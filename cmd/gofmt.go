package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
)

func goFmt(raw []byte) []byte {
	linePrint := func(raw []byte) string {
		var src bytes.Buffer
		s := bufio.NewScanner(bytes.NewReader(raw))
		for line := 1; s.Scan(); line++ {
			fmt.Fprintf(&src, "%5d\t%s\n", line, s.Bytes())
		}
		return src.String()
	}

	fSet := token.NewFileSet()
	ast, err := parser.ParseFile(fSet, "", raw, parser.ParseComments)
	if err != nil {
		log.Fatalf("bad Go source code was generated: %s\n%s", err.Error(), linePrint(raw))
	}

	out := bytes.NewBuffer(nil)
	err = (&printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}).Fprint(out, fSet, ast)
	if err != nil {
		log.Fatalf("generated Go source code could not be reformatted: %s\n%s", err.Error(), linePrint(raw))
	}
	return out.Bytes()
}
