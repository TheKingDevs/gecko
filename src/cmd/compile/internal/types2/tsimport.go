// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gecko TypeScript-style imports.
//
// In .gk files three extra import forms are accepted in addition to the
// standard Go forms:
//
//	import x from "lib"             default import: binds x to the exported
//	                                symbol x of "lib" when present, otherwise
//	                                to the package "lib" itself.
//	import { a, x.y } from "lib"    named imports: bind the exported symbol
//	                                a of "lib"; "x.y" binds the exported
//	                                symbol y of "lib" under the synthetic
//	                                package qualifier x.
//	import x, { a } from "lib"      mixed form combining the two above.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// geckoTSImport resolves a TypeScript-style syntax.ImportDecl. It binds the
// default import (s.LocalPkgName) and the named members (s.TSMembers) into
// fileScope, and registers imp as explicitly imported.
func (check *Checker) geckoTSImport(s *syntax.ImportDecl, fileScope *Scope, imp *Package, pkgImports map[*Package]bool, pkg *Package) {
	// add package to list of explicit imports
	if !pkgImports[imp] {
		pkgImports[imp] = true
		pkg.imports = append(pkg.imports, imp)
	}

	if s.LocalPkgName != nil {
		name := s.LocalPkgName.Value
		if name == "init" {
			check.error(s, InvalidInitDecl, "cannot import package as init - init must be a func")
		} else if obj := lookupExportedMember(imp, name); obj != nil {
			// Default import of an exported symbol (the common gecko case).
			// The declaration is used for diagnostics only; the object is
			// inserted directly like a dot-import to share it across files.
			check.declareGeckoMember(fileScope, s.LocalPkgName, name, obj)
		} else {
			// No exported symbol named name: bind the package itself.
			pkgName := NewPkgName(s.Pos(), pkg, name, imp)
			check.recordDef(s.LocalPkgName, pkgName)
			check.usedPkgNames[pkgName] = true
			check.imports = append(check.imports, pkgName)
			check.declare(fileScope, nil, pkgName, nopos)
		}
	}

	for _, m := range s.TSMembers {
		obj := lookupExportedMember(imp, m.Name.Value)
		if obj == nil {
			check.errorf(m.Name, BadImportPath, "symbol %q not exported by package %q", m.Name.Value, imp.path)
			continue
		}
		if m.Qualifier != nil {
			// Qualified member x.y: expose y of imp through the synthetic
			// package qualifier x, so that the code can write x.y.
			alias := newGeckoAliasPackage(m.Qualifier.Value, m.Name.Value, obj)
			pkgName := NewPkgName(m.Pos(), pkg, m.Qualifier.Value, alias)
			check.recordDef(m.Qualifier, pkgName)
			check.usedPkgNames[pkgName] = true
			check.imports = append(check.imports, pkgName)
			check.declare(fileScope, nil, pkgName, nopos)
			continue
		}
		// Plain member a: bind the exported symbol directly.
		check.declareGeckoMember(fileScope, m.Name, m.Name.Value, obj)
	}

	// Record an implicit package name on the whole import declaration so that
	// clients like the compiler's declCollector (which inspects
	// Info.PkgNameOf(imp).Imported().Path() to detect "embed" and "unsafe")
	// have a non-nil package name to look at, even when the default import
	// binds an exported symbol or the import is named-only.
	if check.Implicits == nil {
		check.Implicits = make(map[syntax.Node]Object)
	}
	check.Implicits[s] = NewPkgName(s.Pos(), pkg, "", imp)
}

// lookupExportedMember returns the exported object named name in pkg's
// scope, or nil if no such object exists. Both Go uppercase names and
// gecko "export" declarations qualify.
func lookupExportedMember(pkg *Package, name string) Object {
	if pkg.scope == nil {
		return nil
	}
	obj := pkg.scope.Lookup(name)
	if obj == nil || !(isExported(name) || resolve(name, obj).Exported()) {
		return nil
	}
	return obj
}

// declareGeckoMember binds obj under name in fileScope, reporting a
// redeclaration if name is already taken. The object is inserted directly
// (like a dot-import) so it may be shared across files.
func (check *Checker) declareGeckoMember(fileScope *Scope, pos poser, name string, obj Object) {
	if alt := fileScope.Lookup(name); alt != nil {
		err := check.newError(DuplicateDecl)
		err.addf(pos, "%s redeclared in this block", obj.Name())
		err.addAltDecl(alt)
		err.report()
		return
	}
	fileScope.insert(name, obj)
}

// newGeckoAliasPackage builds a synthetic package whose scope exposes the
// single object obj under member; used to resolve "x.y" TS import members.
func newGeckoAliasPackage(qual, member string, obj Object) *Package {
	p := NewPackage("gecko.import."+qual+"."+member, qual)
	p.scope = NewScope(Universe, nopos, nopos, qual)
	p.complete = true
	p.fake = true
	p.scope.insert(member, obj)
	return p
}