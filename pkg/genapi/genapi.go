// Copyright 2019 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This code is heavily inspired by prometheus-operator's API doc generation:
// https://github.com/coreos/prometheus-operator/blob/master/cmd/po-docgen/api.go

package genapi

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/doc"
	"go/token"
	"go/types"
	"io"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// WriteMarkdown writes the API of pkg as markdown to w.
func WriteMarkdown(w io.Writer, pkg *Package) error {
	b := bufio.NewWriter(w)
	printHeader(b)
	printTOC(b, pkg)
	printTypes(b, pkg)
	return b.Flush()
}

func printHeader(w io.Writer) {
	fmt.Fprintln(w, "# API")
	fmt.Fprintln(w)
	fmt.Fprint(w, "**Note:** This document is generated from code and comments. Do not edit it directly.")
}

func printTOC(w io.Writer, pkg *Package) {
	fmt.Fprintf(w, "\n## Table of Contents\n")
	for _, name := range sortedNames(pkg) {
		fmt.Fprintf(w, "* %s\n", mdSectionLink(name))
	}
}

func printTypes(w io.Writer, pkg *Package) {
	for _, name := range sortedNames(pkg) {
		if s, ok := pkg.Structs[name]; ok {
			printStruct(w, pkg, s)
		} else {
			printConst(w, pkg, pkg.Constants[name])
		}
	}
}

func printStruct(w io.Writer, pkg *Package, s Struct) {
	fmt.Fprintf(w, "\n## %s\n\n%s\n\n", s.Name, s.Doc)
	fmt.Fprintln(w, "| Field | Description | Type | Required |")
	fmt.Fprintln(w, "| ----- | ----------- | ---- | -------- |")
	for _, f := range s.Fields {
		fmt.Fprintln(w, "|", f.Name, "|", f.Doc, "|", mdType(pkg, f.Type), "|", f.Required, "|")
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "[Back to TOC](#table-of-contents)")
}

func printConst(w io.Writer, pkg *Package, c Constant) {
	fmt.Fprintf(w, "\n## %s\n\n%s\n\n", c.Name, c.Doc)
	fmt.Fprintln(w, "| Name | Value | Description |")
	fmt.Fprintln(w, "| ---- | ----- | ----------- |")
	for _, v := range c.Values {
		fmt.Fprintln(w, "|", v.Name, "|", constant.Val(v.Value), "|", v.Doc, "|")
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "[Back to TOC](#table-of-contents)")
}

func mdSectionLink(name string) string {
	link := strings.ToLower(name)
	link = strings.Replace(link, " ", "-", -1)
	return fmt.Sprintf("[%s](#%s)", name, link)
}

func mdType(pkg *Package, typ ast.Expr) string {
	switch e := typ.(type) {
	case *ast.Ident:
		if _, ok := pkg.Structs[e.Name]; ok {
			return mdSectionLink(e.Name)
		}
		if _, ok := pkg.Constants[e.Name]; ok {
			return mdSectionLink(e.Name)
		}
		return e.Name
	case *ast.SelectorExpr:
		pkgID := e.X.(*ast.Ident).String()
		typID := e.Sel.Name
		text := pkgID + "." + typID
		if path, ok := importPath(pkg, typ, pkgID); ok {
			// https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta
			return fmt.Sprintf("[%s](https://pkg.go.dev/%s#%s)", text, path, typID)
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

func sortedNames(pkg *Package) []string {
	var names []string
	for name := range pkg.Structs {
		names = append(names, name)
	}
	for name := range pkg.Constants {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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

// A Package represents a package.
type Package struct {
	Pkg     *packages.Package
	AstPkg  *ast.Package
	DocPkg  *doc.Package
	FileSet *token.FileSet

	Constants map[string]Constant
	Structs   map[string]Struct
}

// ParsePackage parses the package in the given path.
func ParsePackage(path string) (*Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypes | packages.NeedDeps | packages.NeedImports, // | packages.NeedTypesInfo,
		Fset: token.NewFileSet(),
	}
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("expected 1 package, found %d", len(pkgs))
	}
	if len(pkgs[0].Errors) > 0 {
		return nil, pkgs[0].Errors[0]
	}
	pkg := pkgs[0]
	astPkg := &ast.Package{
		Name:  path,
		Files: make(map[string]*ast.File),
		Scope: ast.NewScope(nil),
	}
	for _, file := range pkg.Syntax {
		name := pkg.Fset.File(file.Package).Name()
		astPkg.Files[name] = file
		for _, obj := range file.Scope.Objects {
			astPkg.Scope.Insert(obj)
		}
	}
	p := &Package{
		FileSet:   cfg.Fset,
		Pkg:       pkg,
		AstPkg:    astPkg,
		DocPkg:    doc.New(astPkg, "", 0),
		Constants: make(map[string]Constant),
		Structs:   make(map[string]Struct),
	}
	// Constants are not necessarily associated with their type.
	// They may be associated with another type or the package.
	scope := pkg.Types.Scope()
	consts := constValues(scope, p.DocPkg.Consts)
	for _, typ := range p.DocPkg.Types {
		if len(typ.Consts) == 0 {
			continue
		}
		for typName, vals := range constValues(scope, typ.Consts) {
			consts[typName] = append(consts[typName], vals...)
		}
	}
	for _, dt := range p.DocPkg.Types {
		if vals, ok := consts[scope.Lookup(dt.Name).Type()]; ok {
			p.Constants[dt.Name] = Constant{
				Doc:    fmtRawDoc(dt.Doc),
				Name:   dt.Name,
				Values: vals,
			}
		} else if st, ok := dt.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType); ok {
			p.Structs[dt.Name] = newStruct(dt, st)
		}
	}
	return p, nil
}

// A Constant represents a constant.
type Constant struct {
	Name   string
	Doc    string
	Values []Value
}

// A Value represents a constant value.
type Value struct {
	Doc   string
	Name  string
	Value constant.Value
}

func constValues(scope *types.Scope, consts []*doc.Value) map[types.Type][]Value {
	cvs := make(map[types.Type][]Value)
	for _, v := range consts {
		docWrap := fmtRawDoc(v.Doc)
		for _, s := range v.Decl.Specs {
			spec, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			doc := fmtRawDoc(spec.Doc.Text())
			if doc == "" {
				doc = docWrap
			}
			for _, n := range spec.Names {
				obj := scope.Lookup(n.Name)
				typ := obj.Type()
				cvs[typ] = append(cvs[typ], Value{
					Doc:   doc,
					Name:  n.Name,
					Value: obj.(*types.Const).Val(),
				})
			}
		}
	}
	return cvs
}

// A Struct represents a struct.
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

// A Field represents a struct field.
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
