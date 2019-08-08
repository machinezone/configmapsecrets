// Copyright 2019 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This code is heavily inspired by prometheus-operator's API doc generation:
// https://github.com/coreos/prometheus-operator/blob/master/cmd/po-docgen/api.go

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func main() {
	flag.Parse()

	pkg, err := ParseDir(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	PrintHeader()
	PrintTOC(pkg)
	PrintStructs(pkg)
}

func PrintHeader() {
	fmt.Println("# API Docs")
	fmt.Println()
	fmt.Print("**Note:** This document is generated from code and comments. Do not edit it directly.")
}

func PrintTOC(pkg *Package) {
	fmt.Printf("\n## Table of Contents\n")
	for _, s := range sortedStructs(pkg.Structs) {
		fmt.Printf("* %s\n", mdSectionLink(s.Name))
	}
}

func PrintStructs(pkg *Package) {
	for _, s := range sortedStructs(pkg.Structs) {
		fmt.Printf("\n## %s\n\n%s\n\n", s.Name, s.Doc)

		fmt.Println("| Field | Description | Type | Required |")
		fmt.Println("| ----- | ----------- | ---- | -------- |")
		for _, f := range s.Fields {
			fmt.Println("|", f.Name, "|", f.Doc, "|", mdType(pkg, f.Type), "|", f.Required, "|")
		}
		fmt.Println("")
		fmt.Println("[Back to TOC](#table-of-contents)")
	}
}

func mdSectionLink(name string) string {
	link := strings.ToLower(name)
	link = strings.Replace(link, " ", "-", -1)
	return fmt.Sprintf("[%s](#%s)", name, link)
}

func mdType(pkg *Package, typ ast.Expr) string {
	switch e := typ.(type) {
	case *ast.Ident:
		if _, ok := pkg.Structs[e.Name]; !ok {
			return e.Name
		}
		return mdSectionLink(e.Name)
	case *ast.SelectorExpr:
		pkgID := e.X.(*ast.Ident).String()
		typID := e.Sel.Name
		text := pkgID + "." + typID
		if path, ok := importPath(pkg, typ, pkgID); ok {
			// https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta
			return fmt.Sprintf("[%s](https://godoc.org/%s#%s)", text, path, typID)
		}
		return text
	case *ast.StarExpr:
		return "*" + mdType(pkg, e.X)
	case *ast.ArrayType:
		return "[]" + mdType(pkg, e.Elt)
	case *ast.MapType:
		return "map[" + mdType(pkg, e.Key) + "]" + mdType(pkg, e.Value)
	default:
		return ""
	}
}

func sortedStructs(set map[string]Struct) []Struct {
	var s []Struct
	for _, v := range set {
		s = append(s, v)
	}
	sort.Slice(s, func(i int, k int) bool { return s[i].Name < s[k].Name })
	return s
}

func importPath(pkg *Package, exp ast.Expr, pkgID string) (string, bool) {
	file, ok := pkg.AstPkg.Files[pkg.FileSet.Position(exp.Pos()).Filename]
	if !ok {
		return "", false
	}
	for _, i := range file.Imports {
		if i.Name != nil {
			if i.Name.Name == pkgID {
				return unquote(i.Path.Value), true
			}
		} else if path.Base(i.Path.Value) == pkgID {
			return unquote(i.Path.Value), true
		}
	}
	return "", false
}

type Package struct {
	FileSet *token.FileSet
	AstPkg  *ast.Package
	DocPkg  *doc.Package

	Structs map[string]Struct
}

func ParseDir(path string) (*Package, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, path, ignoreTests, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected 1 package, found %d", len(pkgs))
	}
	pkg := &Package{FileSet: fset}
	for _, pkgAst := range pkgs {
		pkg.AstPkg = pkgAst
		pkg.DocPkg = doc.New(pkgAst, "", 0)
	}
	pkg.Structs = make(map[string]Struct)
	for _, dt := range pkg.DocPkg.Types {
		if st, ok := dt.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType); ok {
			s := newStruct(dt, st)
			pkg.Structs[s.Name] = s
		}
	}
	return pkg, nil
}

func ignoreTests(f os.FileInfo) bool {
	return !strings.HasSuffix("_test.go", f.Name())
}

type Struct struct {
	Name   string
	Doc    string
	Fields []Field
}

func newStruct(dt *doc.Type, st *ast.StructType) Struct {
	s := Struct{
		Name: dt.Name,
		Doc:  fmtRawDoc(dt.Doc),
	}
	for _, v := range st.Fields.List {
		if f, ok := newField(v); ok {
			s.Fields = append(s.Fields, f)
		}
	}
	return s
}

type Field struct {
	Name     string
	Doc      string
	Type     ast.Expr
	Required bool
}

func newField(f *ast.Field) (Field, bool) {
	// TODO: Do we care about multi-field lines? We only take the first one.
	// type Foo struct {
	//  Bar, Baz string  // Baz is dropped
	// }
	name, opts := jsonTag(f.Tag)
	if containsString(opts, "inline") {
		return Field{}, false
	}
	if name == "" {
		if len(f.Names) == 0 {
			name = f.Type.(*ast.Ident).Name // embedded
		} else {
			name = f.Names[0].Name
		}
	}
	required := true
	if containsString(opts, "omitempty") ||
		(f.Doc != nil && hasComment(f.Doc.List, "+optional")) {
		required = false
	}
	return Field{
		Name:     name,
		Doc:      fmtRawDoc(f.Doc.Text()),
		Type:     f.Type,
		Required: required,
	}, true
}

func unquote(s string) string {
	v, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return v
}

func jsonTag(tag *ast.BasicLit) (name string, opts []string) {
	if tag == nil {
		return "", nil
	}
	opts = strings.Split(reflect.StructTag(unquote(tag.Value)).Get("json"), ",")
	return opts[0], opts[1:]
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func hasComment(comments []*ast.Comment, comment string) bool {
	for _, s := range comments {
		if strings.TrimSpace(s.Text) == comment {
			return true
		}
	}
	return false
}

// fmtRawDoc is copy/pasted from prometheus-operator:
// https://github.com/coreos/prometheus-operator/blob/master/cmd/po-docgen/api.go
func fmtRawDoc(rawDoc string) string {
	var buffer bytes.Buffer
	delPrevChar := func() {
		if buffer.Len() > 0 {
			buffer.Truncate(buffer.Len() - 1) // Delete the last " " or "\n"
		}
	}

	// Ignore all lines after ---
	rawDoc = strings.Split(rawDoc, "---")[0]

	for _, line := range strings.Split(rawDoc, "\n") {
		line = strings.TrimRight(line, " ")
		leading := strings.TrimLeft(line, " ")
		switch {
		case len(line) == 0: // Keep paragraphs
			delPrevChar()
			buffer.WriteString("\n\n")
		case strings.HasPrefix(leading, "TODO"): // Ignore one line TODOs
		case strings.HasPrefix(leading, "+"): // Ignore instructions to go2idl
		default:
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				delPrevChar()
				line = "\n" + line + "\n" // Replace it with newline. This is useful when we have a line with: "Example:\n\tJSON-someting..."
			} else {
				line += " "
			}
			buffer.WriteString(line)
		}
	}

	postDoc := strings.TrimRight(buffer.String(), "\n")
	postDoc = strings.Replace(postDoc, "\\\"", "\"", -1) // replace user's \" to "
	postDoc = strings.Replace(postDoc, "\"", "\\\"", -1) // Escape "
	postDoc = strings.Replace(postDoc, "\n", "\\n", -1)
	postDoc = strings.Replace(postDoc, "\t", "\\t", -1)
	postDoc = strings.Replace(postDoc, "|", "\\|", -1)

	return postDoc
}
