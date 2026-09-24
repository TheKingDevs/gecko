// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements type checking of gecko class declarations and
// `new` expressions. The parser desugars `class` declarations into a
// TypeDecl whose Type is a *syntax.ClassType plus one FuncDecl per class
// method (with an implicit `this` receiver), so most of the machinery here
// only deals with synthesizing the struct underlying a class and with
// semantics of `new Class(args)`.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// geckoGetterResultType returns the type of class field `field` as seen
// through the receiver type recv of a synthesized accessor method
// (`geckoGet_<field>`), or nil if no such field is resolvable. The method
// has no declared result type; funcDecl uses this to fill it in with the
// class field's inferred type.
func (check *Checker) geckoGetterResultType(recv Type, field string) Type {
	obj, _, _ := lookupFieldOrMethod(recv, true, check.pkg, field, false)
	v, _ := obj.(*Var)
	if v == nil {
		return nil
	}
	return v.typ
}

// geckoIfaceFieldGetter returns the synthesized accessor (geckoGet_<name>)
// for the interface field `name` of a value of type typ, or nil unless the
// value's type is (a pointer to) an interface with such a field member.
func (check *Checker) geckoIfaceFieldGetter(typ Type, name string) *Func {
	if isInterfacePtr(typ) {
		return nil
	}
	u := typ
	if p, ok := typ.(*Pointer); ok {
		u = p.base
	}
	ityp, ok := u.Underlying().(*Interface)
	if !ok {
		return nil
	}
	return ityp.geckoFieldGetter(name)
}

// geckoClassType computes the underlying *Struct for a gecko class type.
//
// Field names are collected in source order from `this.<name>` selector
// expressions in the class methods, with the constructor processed first.
// Field types are inferred from the first plain assignment to the field in
// the constructor body:
//
//   - a literal RHS determines the type (bool, int, float64, rune,
//     complex128, or string);
//   - a constructor parameter with an explicit type propagates that type;
//   - anything else (untyped constructor parameters, assignments in other
//     methods, reads without prior assignment) yields `any`.
func (check *Checker) geckoClassType(cls *syntax.ClassType) *Struct {
	if s, ok := check.geckoClass[cls]; ok {
		return s
	}

	// Resolve the base class, if any. The derived class's underlying
	// struct embeds the base by value as its first field, so the base's
	// fields and methods are promoted and `new Derived(...)` allocates
	// both halves. The parser already rewrote `super` references to
	// `this.<base>.<...>` selections on the embedded field.
	var baseType *Named
	var baseFields map[string]bool
	if cls.Base != nil {
		baseType = check.geckoBaseClass(cls.Base)
		if baseType != nil {
			baseFields = make(map[string]bool)
			// The embedded base field itself (`this.<base>`) plus every
			// field reachable through it (direct and promoted) is not
			// redeclared on the derived struct.
			baseFields[baseType.Obj().Name()] = true
			check.geckoCollectFieldNames(baseType, baseFields)
		}
	}
	baseField := func(name string) bool {
		if baseFields == nil {
			return false
		}
		return baseFields[name]
	}

	// Process the constructor first so field types are determined before
	// any other method body references them.
	methods := cls.Methods
	if len(methods) > 1 && methods[0].Name.Value != "constructor" {
		for i, m := range methods[1:] {
			if m.Name.Value == "constructor" {
				methods[0], methods[i+1] = m, methods[0]
				break
			}
		}
	}

	// Collect constructor parameter declarations for type propagation.
	ctorParams := make(map[string]*syntax.Field)
	if len(methods) > 0 && methods[0].Name.Value == "constructor" {
		for _, f := range methods[0].Type.ParamList {
			if f.Name != nil {
				ctorParams[f.Name.Value] = f
			}
		}
	}

	type fieldInfo struct {
		pos  syntax.Pos
		name string
		rhs  syntax.Expr // RHS of first plain assignment in constructor
	}
	var infos []*fieldInfo
	fieldIndex := make(map[string]int)

	addField := func(sel *syntax.Name, rhs syntax.Expr) {
		if baseField(sel.Value) {
			// A promoted (base-class) field; it lives on the embedded
			// base, not on this struct.
			return
		}
		i, ok := fieldIndex[sel.Value]
		if !ok {
			fieldIndex[sel.Value] = len(infos)
			infos = append(infos, &fieldInfo{pos: sel.Pos(), name: sel.Value})
			i = len(infos) - 1
		}
		if infos[i].rhs == nil && rhs != nil {
			infos[i].rhs = rhs
		}
	}

	isSelf := func(x syntax.Expr) bool {
		if n, ok := syntax.Unparen(x).(*syntax.Name); ok && n.Value == "this" {
			return true
		}
		return false
	}

	walkBody := func(body *syntax.BlockStmt) {
		// Selector expressions that are the `Fun` of a call expression are
		// method calls (`this.m(arg)`), not field references.
		isCallTarget := make(map[*syntax.SelectorExpr]bool)
		syntax.Inspect(body, func(n syntax.Node) bool {
			if cl, ok := n.(*syntax.CallExpr); ok {
				if sel, ok := syntax.Unparen(cl.Fun).(*syntax.SelectorExpr); ok {
					isCallTarget[sel] = true
				}
			}
			return true
		})

		syntax.Inspect(body, func(n syntax.Node) bool {
			if n == nil {
				return false
			}
			switch n := n.(type) {
			case *syntax.SelectorExpr:
				if isSelf(n.X) && !isCallTarget[n] {
					addField(n.Sel, nil)
				}
			case *syntax.AssignStmt:
				if n.Op == 0 && n.Rhs != nil {
					if sel, ok := n.Lhs.(*syntax.SelectorExpr); ok && isSelf(sel.X) && !isCallTarget[sel] {
						// `null` contributes no field type (mirroring geckoUnify):
						// record the field but let a later non-null assignment in
						// the constructor determine its type.
						if nm, isNull := syntax.Unparen(n.Rhs).(*syntax.Name); isNull && nm.Value == "null" {
							addField(sel.Sel, nil)
						} else {
							addField(sel.Sel, n.Rhs)
						}
					}
				}
			}
			return true
		})
	}

	for _, m := range methods {
		walkBody(m.Body)
	}

	// Fields and consts declared in the class body (`var x = v`,
	// `const X = v`, or the bare `x = v` shorthand) participate in field
	// inference like any constructor assignment — the parser already merged
	// them into the constructor. A declaration with an explicit `: T`
	// annotation, or without an initializer, is seeded here so the field
	// exists even when no `this.<name>` selector appears elsewhere.
	addFieldInfo := func(nm *syntax.Name, rhs syntax.Expr) {
		if baseField(nm.Value) {
			return
		}
		i, ok := fieldIndex[nm.Value]
		if !ok {
			fieldIndex[nm.Value] = len(infos)
			infos = append(infos, &fieldInfo{pos: nm.Pos(), name: nm.Value})
			i = len(infos) - 1
		}
		if infos[i].rhs == nil && rhs != nil {
			infos[i].rhs = rhs
		}
	}

	declType := make(map[string]Type)
	for _, d := range cls.Fields {
		if d.Type != nil {
			declType[d.NameList[0].Value] = check.typ(d.Type)
		}
		rhs := d.Values
		if fv := syntax.UnpackListExpr(d.Values); len(fv) == 1 {
			rhs = fv[0]
		}
		if d.Type == nil {
			addFieldInfo(d.NameList[0], rhs)
		} else {
			addFieldInfo(d.NameList[0], nil)
		}
	}
	consts := make(map[string]bool)
	for _, d := range cls.Consts {
		if d.Type != nil {
			declType[d.NameList[0].Value] = check.typ(d.Type)
		}
		rhs := d.Values
		if fv := syntax.UnpackListExpr(d.Values); len(fv) == 1 {
			rhs = fv[0]
		}
		addFieldInfo(d.NameList[0], rhs)
		consts[d.NameList[0].Value] = true
		if rhs != nil {
			if check.geckoConstRhs == nil {
				check.geckoConstRhs = make(map[syntax.Expr]bool)
			}
			check.geckoConstRhs[rhs] = true
		}
	}

	strct := new(Struct)
	embedded := 0
	if baseType != nil {
		embedded = 1
	}
	strct.fields = make([]*Var, embedded+len(infos))
	if baseType != nil {
		// Embed the base class by value so its fields and methods are
		// promoted. The derived struct allocates the base instance
		// inline; the base constructor runs through the rewritten
		// `this.<base>.constructor(...)` super call.
		f := NewField(cls.Base.Pos(), baseType.Obj().Pkg(), baseType.Obj().Name(), baseType, true)
		f.SetGeckoExported(true)
		strct.fields[0] = f
	}
	for i, info := range infos {
		t := check.geckoFieldType(info.rhs, ctorParams)
		if dt, ok := declType[info.name]; ok {
			t = dt
		}
		strct.fields[embedded+i] = NewField(info.pos, check.pkg, info.name, t, false)
	}

	if check.geckoClass == nil {
		check.geckoClass = make(map[*syntax.ClassType]*Struct)
	}
	check.geckoClass[cls] = strct

	if len(consts) > 0 {
		if check.geckoConstFields == nil {
			check.geckoConstFields = make(map[*Struct]map[string]bool)
		}
		check.geckoConstFields[strct] = consts
	}

	return strct
}

// geckoBaseClass resolves the base class of a derived gecko class and
// returns its named type. The base must be a class type: a same-package
// name or a qualified `pkg.Class` selector, mirroring geckoNewType.
// It reports nil (with a diagnostic) when the base does not resolve to a
// class type, which covers recursive inheritance: a base whose declaration
// is still being type-checked (grey) is reported here, before types2's
// cycle machinery forces its underlying type in a state that cannot be
// inspected.
func (check *Checker) geckoBaseClass(e syntax.Expr) *Named {
	var obj Object
	switch r := syntax.Unparen(e).(type) {
	case *syntax.Name:
		obj = check.lookup(r.Value)
	case *syntax.SelectorExpr:
		if x, ok := syntax.Unparen(r.X).(*syntax.Name); ok {
			if pkgobj, ok := check.lookup(x.Value).(*PkgName); ok {
				// A qualified base class counts as a use of the import.
				check.usedPkgNames[pkgobj] = true
				obj = pkgobj.imported.Scope().Lookup(r.Sel.Value)
				if obj != nil && obj.Pkg() != pkgobj.imported {
					obj = nil
				}
			}
		}
	}
	tn, ok := obj.(*TypeName)
	if !ok {
		if obj == nil {
			check.errorf(e, InvalidSyntaxTree, "undefined base class %s", e)
		} else {
			check.errorf(e, InvalidSyntaxTree, "%s is not a class type", e)
		}
		return nil
	}
	check.objDecl(tn)
	if _, inProgress := check.objPathIdx[tn]; inProgress {
		// The base declaration is part of a declaration cycle (R1
		// extends R2 and R2 extends R1, or a class extending itself).
		// Report it here with a clear message and bail out; forcing the
		// underlying type now would hit the grey object.
		check.errorf(e, InvalidSyntaxTree, "cyclic inheritance with class %s", tn.Name())
		return nil
	}
	named, _ := tn.typ.(*Named)
	if named == nil {
		return nil
	}
	if _, ok := named.Underlying().(*Struct); !ok {
		check.errorf(e, InvalidSyntaxTree, "%s is not a class type", e)
		return nil
	}
	return named
}

// geckoCollectFieldNames records every field name reachable on the class
// type c (its own struct fields as well as those promoted through
// embedded base fields) into set.
func (check *Checker) geckoCollectFieldNames(c *Named, set map[string]bool) {
	strct, ok := c.Underlying().(*Struct)
	if !ok {
		return
	}
	for i := 0; i < strct.NumFields(); i++ {
		f := strct.Field(i)
		set[f.Name()] = true
		if f.Embedded() {
			if embedded, ok := f.Type().(*Named); ok {
				check.geckoCollectFieldNames(embedded, set)
			}
		}
	}
}

// geckoFieldType infers the type of a class field from the RHS of its first
// constructor assignment, if any. See geckoClassType for the rules.
func (check *Checker) geckoFieldType(rhs syntax.Expr, ctorParams map[string]*syntax.Field) Type {
	switch r := rhs.(type) {
	case *syntax.BasicLit:
		switch r.Kind {
		case syntax.IntLit:
			return Typ[Int]
		case syntax.FloatLit:
			return Typ[Float64]
		case syntax.RuneLit:
			return Typ[Rune]
		case syntax.ImagLit:
			return Typ[Complex128]
		case syntax.StringLit:
			return Typ[String]
		}
	case *syntax.Name:
		switch r.Value {
		case "true", "false":
			return Typ[Bool]
		case "null":
			// `null` carries no type information; any accepts it.
			return universeAny.Type()
		default:
			if p, ok := ctorParams[r.Value]; ok {
				if p.Type != nil {
					return check.typ(p.Type)
				}
			}
		}
	case *syntax.Operation:
		// A signed numeric literal (e.g. `-1`) is a unary operation over a
		// basic literal; infer the literal's type.
		if r.Y == nil && (r.Op == syntax.Add || r.Op == syntax.Sub) {
			if lit, ok := syntax.Unparen(r.X).(*syntax.BasicLit); ok {
				if t := check.geckoFieldType(lit, ctorParams); t != universeAny.Type() {
					return t
				}
			}
		}
	case *syntax.NewExpr:
		// `this.x = new SomeClass()` stores a class instance; infer the
		// class's pointer type so selector chains keep working.
		if t := check.geckoNewType(r.Type); t != nil {
			return t
		}
	}
	return universeAny.Type()
}

// geckoNewType resolves the pointer type produced by a `new` expression for
// field type inference. It resolves the class name structurally (without
// type-checking the class declaration) so that self-referential shapes such
// as linked lists do not recurse into the class's own inference.
func (check *Checker) geckoNewType(e syntax.Expr) Type {
	var obj Object
	switch r := syntax.Unparen(e).(type) {
	case *syntax.Name:
		obj = check.lookup(r.Value)
	case *syntax.SelectorExpr:
		// Qualified class name: pkg.Class. Resolve the package object and
		// look up the class in its package scope.
		if x, ok := syntax.Unparen(r.X).(*syntax.Name); ok {
			if pkgobj, ok := check.lookup(x.Value).(*PkgName); ok {
				obj = pkgobj.imported.Scope().Lookup(r.Sel.Value)
				if obj != nil && obj.Pkg() != pkgobj.imported {
					obj = nil
				}
			}
		}
	}
	if tn, ok := obj.(*TypeName); ok {
		if named, ok := tn.typ.(*Named); ok {
			return NewPointer(named)
		}
	}
	return nil
}

// newExpr type checks a gecko `new Class(args)` expression. The result is a
// value of pointer type *Class. The class's `constructor` method, if any, is
// called on the newly allocated value with the arguments converted to its
// parameter types; the constructor's result value (if any) is discarded.
func (check *Checker) newExpr(x *operand, e *syntax.NewExpr) {
	check.exprOrType(x, e.Type, true)
	if x.mode() == invalid {
		check.use(e.ArgList...)
		x.expr = e
		return
	}
	if x.mode() != typexpr {
		check.errorf(e.Type, NotAType, "%s is not a class type", e.Type)
		check.use(e.ArgList...)
		x.invalidate()
		x.expr = e
		return
	}

	named, isNamed := x.typ().(*Named)
	if !isNamed {
		check.errorf(e.Type, InvalidSyntaxTree, "cannot use new with %s: not a class type", e.Type)
		check.use(e.ArgList...)
		x.invalidate()
		x.expr = e
		return
	}

	ptr := NewPointer(named)
	noCtor := len(e.ArgList) != 0
	if ctor, _, _ := lookupFieldOrMethod(ptr, true, check.pkg, "constructor", false); ctor != nil {
		noCtor = false
		check.objDecl(ctor) // ensure fully set-up signature
		sig := ctor.Type().(*Signature)

		call := &syntax.CallExpr{Fun: e.Type, ArgList: e.ArgList}
		call.HasDots = e.HasDots
		if e.HasDots {
			check.error(e, NonVariadicDotDotDot, "cannot use ... in new expression")
		}

		targetAt := func(i int) *target { return newTarget(sig.argType(i), "constructor argument") }
		args, atargs := check.genericExprList(targetAt, call.ArgList)
		check.arguments(call, sig, nil, nil, args, atargs)
	}
	if noCtor {
		check.errorf(e, WrongArgCount, "too many arguments in new expression (class %s has no constructor)", named.obj.Name())
		check.use(e.ArgList...)
	}

	x.mode_ = value
	x.typ_ = ptr
	x.expr = e
}

// geckoConstFieldGuard rejects assignments to read-only (const) class
// fields. The class's underlying struct comes from the current method's
// `this` receiver; the only write permitted is the initializer the parser
// synthesized for the const declaration (identified by its RHS expression).
func (check *Checker) geckoConstFieldGuard(lhs, rhs syntax.Expr) {
	if check.geckoConstFields == nil {
		return
	}
	sel, ok := syntax.Unparen(lhs).(*syntax.SelectorExpr)
	if !ok {
		return
	}
	self, ok := syntax.Unparen(sel.X).(*syntax.Name)
	if !ok || self.Value != "this" {
		return
	}
	var strct *Struct
	if sig := check.sig; sig != nil && sig.recv != nil {
		if ptr, ok := sig.recv.typ.(*Pointer); ok {
			if named, ok := ptr.base.(*Named); ok {
				strct, _ = named.underlying.(*Struct)
			}
		}
	}
	if strct == nil {
		return
	}
	consts := check.geckoConstFields[strct]
	if consts == nil || !consts[sel.Sel.Value] {
		return
	}
	if rhs != nil && check.geckoConstRhs[rhs] {
		return
	}
	check.errorf(lhs, UnassignableOperand, invalidOp+"cannot assign to read-only (const) class field %s", sel.Sel.Value)
}
