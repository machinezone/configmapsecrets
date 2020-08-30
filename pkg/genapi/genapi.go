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
	"net/url"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/machinezone/configmapsecrets/pkg/genapi/internal"
	"github.com/machinezone/configmapsecrets/pkg/genapi/internal/jsontags"
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

func mdType(pkg *Package, typ types.Type) string {
	switch t := typ.(type) {
	case *types.Basic:
		return t.String()
	case *types.Pointer:
		return "*" + mdType(pkg, t.Elem())
	case *types.Slice:
		return "[]" + mdType(pkg, t.Elem())
	case *types.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), mdType(pkg, t.Elem()))
	case *types.Map:
		return "map[" + mdType(pkg, t.Key()) + "]" + mdType(pkg, t.Elem())
	case *types.Named:
		name := t.Obj().Name()
		switch pkgPath := t.Obj().Pkg().Path(); pkgPath {
		case pkg.Pkg.PkgPath:
			if _, ok := pkg.Structs[name]; ok {
				return mdSectionLink(name)
			}
			if _, ok := pkg.Constants[name]; ok {
				return mdSectionLink(name)
			}
			return name
		default:
			text := packageIdent(pkgPath) + "." + name
			return fmt.Sprintf("[%s](https://pkg.go.dev/%s#%s)", text, pkgPath, name)
		}
	default:
		return "???"
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

func packageIdent(pkg string) string {
	base := path.Base(pkg)
	parent := path.Base(path.Dir(pkg))
	if !isVersion(base) || parent == "." || parent == ".." || parent == "/" {
		return base
	}
	return parent + base
}

var versionRE = regexp.MustCompile("^v([0-9]+)((alpha|beta)([0-9]+))?$")

// isVersion returns a value indicating whether s is a version string.
func isVersion(s string) bool {
	return versionRE.MatchString(s)
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
	fset := token.NewFileSet()
	pkg, pkgs, err := internal.LoadPackage(path, fset)
	if err != nil {
		return nil, err
	}

	// Constants are not necessarily associated with their type.
	// They may be associated with another type or the package.
	scope := pkg.Pkg.Types.Scope()
	consts := constValues(scope, pkg.DocPkg.Consts)
	for _, docType := range pkg.DocPkg.Types {
		if len(docType.Consts) == 0 {
			continue
		}
		for typesType, vals := range constValues(scope, docType.Consts) {
			consts[typesType] = append(consts[typesType], vals...)
		}
	}
	constants := make(map[string]Constant)
	for name, typ := range pkg.Basics {
		if vals, ok := consts[typ.Named]; ok {
			constants[name] = Constant{
				Doc:    fmtRawDoc(typ.DocType.Doc),
				Name:   typ.DocType.Name,
				Values: vals,
			}
		}
	}

	structs := make(map[string]Struct)
	for name, typ := range pkg.Structs {
		structs[name] = Struct{
			Name:   typ.DocType.Name,
			Doc:    fmtRawDoc(typ.DocType.Doc),
			Fields: structFields(pkgs, typ),
		}
	}

	return &Package{
		FileSet:   fset,
		Pkg:       pkg.Pkg,
		AstPkg:    pkg.AstPkg,
		DocPkg:    pkg.DocPkg,
		Constants: constants,
		Structs:   structs,
	}, nil
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

func constValues(scope *types.Scope, consts []*doc.Value) map[*types.Named][]Value {
	cvs := make(map[*types.Named][]Value)
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
				typ := obj.Type().(*types.Named)
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

// A Field represents a struct field.
type Field struct {
	Name     string
	Doc      string
	Type     types.Type
	Required bool
}

func structFields(pkgs map[string]*internal.Package, s *internal.Struct) []Field {
	var list []Field
	for i, n := 0, s.Struct.NumFields(); i < n; i++ {
		f := s.Struct.Field(i)
		if !f.Exported() {
			continue
		}
		doc := s.FieldDoc(f.Name())
		tag := jsontags.Parse(reflect.StructTag(s.Struct.Tag(i)).Get("json"))
		if tag.Contains("inline") {
			o := f.Type().(*types.Named).Obj()
			s := pkgs[o.Pkg().Path()].Structs[o.Name()]
			list = append(list, structFields(pkgs, s)...)
			continue
		}
		name := tag.Name
		if name == "" {
			name = f.Name()
		}
		required := true
		if tag.Contains("omitempty") || hasComment(doc, "+optional") {
			required = false
		}
		list = append(list, Field{
			Name:     name,
			Doc:      fmtRawDoc(doc.Text()),
			Type:     f.Type(),
			Required: required,
		})
	}
	return list
}

func hasComment(grp *ast.CommentGroup, comment string) bool {
	if grp == nil {
		return false
	}
	for _, s := range grp.List {
		if strings.TrimSpace(s.Text) == comment {
			return true
		}
	}
	return false
}

// fmtRawDoc was originally copy/pasted from prometheus-operator, but has diverged:
// https://github.com/coreos/prometheus-operator/blob/master/cmd/po-docgen/api.go
func fmtRawDoc(rawDoc string) string {
	var buffer bytes.Buffer
	trimSpace := func() {
		for {
			r, n := utf8.DecodeLastRune(buffer.Bytes())
			if r == utf8.RuneError || !unicode.IsSpace(r) {
				return
			}
			buffer.Truncate(buffer.Len() - n)
		}
	}

	// Ignore all lines after ---
	rawDoc = strings.Split(rawDoc, "---")[0]

	for _, line := range strings.Split(rawDoc, "\n") {
		line = strings.TrimRight(line, " ")
		leading := strings.TrimLeft(line, " ")
		switch {
		case len(line) == 0: // Keep paragraphs
			trimSpace()
			buffer.WriteString("\n\n")
		case strings.HasPrefix(leading, "TODO"): // Ignore one line TODOs
		case strings.HasPrefix(leading, "+"): // Ignore instructions to go2idl
		default:
			if strings.HasPrefix(line, "More info:") {
				suffix := strings.TrimPrefix(line, "More info:")
				suffix = strings.TrimSuffix(suffix, ".")
				suffix = strings.TrimSpace(suffix)
				if _, err := url.Parse(suffix); err == nil {
					line = "[More info](" + suffix + ")."
				}
			} else if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				trimSpace()
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
	postDoc = strings.Replace(postDoc, "\n", "<br/>", -1)
	postDoc = strings.Replace(postDoc, "\t", "\\t", -1)
	postDoc = strings.Replace(postDoc, "|", "\\|", -1)

	return postDoc
}
