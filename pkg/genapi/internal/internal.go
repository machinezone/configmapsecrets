// Copyright 2020 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// Package represents a package.
type Package struct {
	Pkg     *packages.Package
	AstPkg  *ast.Package
	DocPkg  *doc.Package
	Basics  map[string]*Basic
	Structs map[string]*Struct
}

// LoadPackage loads the package with the given path and all of its dependencies.
func LoadPackage(path string, fset *token.FileSet) (*Package, map[string]*Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedName | packages.NeedTypes | packages.NeedDeps | packages.NeedImports,
		Fset: fset,
	}
	loaded, err := packages.Load(cfg, path)
	if err != nil {
		return nil, nil, err
	}
	if len(loaded) != 1 {
		return nil, nil, fmt.Errorf("expected 1 package, found %d", len(loaded))
	}
	if len(loaded[0].Errors) > 0 {
		return nil, nil, loaded[0].Errors[0]
	}
	pkgs := Packages(loaded)
	pkg := pkgs[loaded[0].PkgPath]
	return pkg, pkgs, nil
}

// Packages returns the internal representation of the loaded packages.
func Packages(loaded []*packages.Package) map[string]*Package {
	pkgs := make(map[string]*Package)
	packages.Visit(loaded, nil, func(p *packages.Package) {
		astPkg := &ast.Package{
			Name:  p.PkgPath,
			Files: make(map[string]*ast.File),
			Scope: ast.NewScope(nil),
		}
		for _, file := range p.Syntax {
			name := p.Fset.File(file.Package).Name()
			astPkg.Files[name] = file
			for _, obj := range file.Scope.Objects {
				astPkg.Scope.Insert(obj)
			}
		}
		docPkg := doc.New(astPkg, "", 0)
		xPkg := &Package{
			Pkg:     p,
			AstPkg:  astPkg,
			DocPkg:  docPkg,
			Basics:  make(map[string]*Basic),
			Structs: make(map[string]*Struct),
		}
		pkgs[p.PkgPath] = xPkg
		scope := p.Types.Scope()
		for _, dt := range docPkg.Types {
			named, ok := scope.Lookup(dt.Name).Type().(*types.Named)
			if !ok {
				continue
			}
			switch typ := named.Underlying().(type) {
			case *types.Basic:
				xPkg.Basics[dt.Name] = &Basic{
					DocType: dt,
					Named:   named,
					Basic:   typ,
				}
			case *types.Struct:
				st, ok := dt.Decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
				if !ok {
					// TODO: handle aliases
					continue
				}
				xPkg.Structs[dt.Name] = &Struct{
					DocType:   dt,
					Named:     named,
					Struct:    typ,
					AstStruct: st,
				}
			}
		}
	})
	return pkgs
}

// Basic represents a basic type.
type Basic struct {
	DocType *doc.Type
	Named   *types.Named
	Basic   *types.Basic
}

// Struct represents a struct type.
type Struct struct {
	DocType   *doc.Type
	Named     *types.Named
	Struct    *types.Struct
	AstStruct *ast.StructType
}

// AstField returns the named ast.Field if it exists.
func (x *Struct) AstField(name string) *ast.Field {
	for _, f := range x.AstStruct.Fields.List {
		if len(f.Names) == 0 { // embedded
			switch t := f.Type.(type) {
			case *ast.Ident:
				if t.Name == name {
					return f
				}
			case *ast.SelectorExpr:
				if t.Sel.Name == name {
					return f
				}
			}
			continue
		}
		for _, n := range f.Names {
			if n.Name == name {
				return f
			}
		}
	}
	return nil
}

// FieldDoc returns the named field's doc if it exists.
func (x *Struct) FieldDoc(name string) *ast.CommentGroup {
	if f := x.AstField(name); f != nil {
		return f.Doc
	}
	return nil
}
