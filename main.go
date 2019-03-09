package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/ast/astutil"
)

type pathnameFlag string

func (f *pathnameFlag) Set(pathname string) error {
	if pathname == "" {
		return errors.New("please don't specify an empty pathname")
	}

	*f = pathnameFlag(path.Clean(pathname))

	return nil
}

func (f *pathnameFlag) String() string { return string(*f) }

var (
	flagToolsFilePathname pathnameFlag
	flagInit              bool
	flagToolToAdd         string
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("toolman: ")

	flag.Var(&flagToolsFilePathname, "f", "/path/to/tools.go")
	flag.BoolVar(&flagInit, "init", false, "create tools.go")
	flag.StringVar(&flagToolToAdd, "add", "", "example.com/some/tool/url")
	flag.Parse()

	if flagToolsFilePathname.String() == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(errors.Wrap(err, "unable to get current working directory"))
		}
		err = flagToolsFilePathname.Set(path.Join(cwd, "tools.go"))
		if err != nil {
			log.Fatal(errors.Wrap(err, "something is very wrong"))
		}
	}

	if flagInit {
		toolsfile, err := os.OpenFile(flagToolsFilePathname.String(), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			log.Fatal(errors.Wrap(err, "cannot open tools file"))
		}

		_, err = toolsfile.WriteString(`// +build toolman
//go:generate go run github.com/execjosh/toolman

package toolman
`)
		if err != nil {
			log.Fatal(errors.Wrap(err, "cannot write tools file"))
		}

		err = toolsfile.Close()
		if err != nil {
			log.Fatal(errors.Wrap(err, "cannot close tools file"))
		}

		return
	}

	toolsfile, err := os.Open(flagToolsFilePathname.String())
	if err != nil {
		log.Fatal(errors.Wrap(err, "cannot open tools file"))
	}
	fset, f, err := parse(toolsfile)
	if err != nil {
		log.Fatal(errors.Wrap(err, "cannot parse tools file"))
	}
	if err := toolsfile.Close(); err != nil {
		log.Fatal(err)
	}

	var toolpaths []string

	if flagToolToAdd != "" {
		err := addImport(fset, f, flagToolToAdd)
		if err != nil {
			log.Fatal(err)
		}
		err = writeTools(fset, f, flagToolsFilePathname.String())
		if err != nil {
			log.Fatal(err)
		}
		toolpaths = []string{flagToolToAdd}
	} else {
		toolpaths, err = extractImports(fset, f)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := installTools(toolpaths...); err != nil {
		log.Fatal(err)
	}
}

func parse(r io.Reader) (*token.FileSet, *ast.File, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to read data")
	}

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, "tools.go", data, parser.ParseComments)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse tools file")
	}

	return fset, f, nil
}

func addImport(fset *token.FileSet, f *ast.File, importpath string) error {
	if !astutil.AddNamedImport(fset, f, "_", importpath) {
		return errors.Errorf("already tracking: %s", flagToolToAdd)
	}
	ast.SortImports(fset, f)
	sort.Slice(f.Imports, func(i, j int) bool {
		return f.Imports[i].Path.Value < f.Imports[j].Path.Value
	})
	return nil
}

// parseToolsFile parses a Go source file and extracts its imports.
// The imports are sorted and deduped.
func extractImports(fset *token.FileSet, f *ast.File) ([]string, error) {
	ast.SortImports(fset, f)

	imports := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, errors.Wrap(err, "cannot unquote value")
		}
		imports = append(imports, p)
	}

	return imports, nil
}

// installTools runs `go install` serially the specified tools.
func installTools(tools ...string) error {
	for _, t := range tools {
		log.Println("installing", t)
		cmd := exec.Command("go", "install", t)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			return errors.Wrap(err, "unable to install")
		}
	}

	return nil
}

func writeTools(fset *token.FileSet, f *ast.File, pathname string) (err error) {
	var buf bytes.Buffer
	err = printer.Fprint(&buf, fset, f)
	if err != nil {
		return errors.Wrap(err, "cannot generate tools file")
	}

	fout, err := os.OpenFile(pathname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrap(err, "cannot open tools file for writing")
	}

	err = format.Node(fout, fset, f)
	if err != nil {
		return errors.Wrap(err, "cannot write to tools file")
	}

	err = fout.Close()
	if err != nil {
		return errors.Wrap(err, "cannot close tools file")
	}

	return nil
}
