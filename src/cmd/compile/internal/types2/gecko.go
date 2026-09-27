// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the gecko declare-or-assign helpers used by
// the checker. The go/types mirror lives in go/types/gecko.go and is
// maintained by hand because the generated source must not reference
// syntax-specific position types.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	. "internal/types/errors"
	"strings"
)

// geckoFile reports whether the position pos belongs to a gecko (.gk)
// source file.
func (check *Checker) geckoFile(pos syntax.Pos) bool {
	base := pos.FileBase()
	return base != nil && strings.HasSuffix(base.Filename(), ".gk")
}

// geckoDynamicFile reports whether pos belongs to a gecko source file that
// runs in "dynamic" mode, i.e. whether gecko's dynamic-typing features apply
// at pos. A "typed" project (Config.GeckoTyped) disables them.
func (check *Checker) geckoDynamicFile(pos syntax.Pos) bool {
	return check.geckoFile(pos) && !check.conf.GeckoTyped
}

// geckoNewVar reports whether any name in lhs is not currently visible
// anywhere in the lexical environment, i.e. whether a gecko `x = e`
// assignment statement one of its lhs names would have to declare.
func (check *Checker) geckoNewVar(lhs []syntax.Expr) bool {
	for _, l := range lhs {
		if ident, ok := syntax.Unparen(l).(*syntax.Name); ok && ident.Value != "_" && check.lookup(ident.Value) == nil {
			return true
		}
	}
	return false
}

// geckoCompositeType infers the type of a dynamic gecko composite literal
// (typed `[1, 2, 3]` or `{"name": "gecko"}`) from its elements. A list with
// key-value pairs is a map; anything else is a slice. Element, key and value
// types come from the element literals themselves; if elements disagree, or
// a non-literal expression appears, the element (or key/value) type is `any`.
func (check *Checker) geckoCompositeType(e *syntax.CompositeLit) (Type, Type) {
	hasKey := false
	for _, el := range e.ElemList {
		if _, ok := el.(*syntax.KeyValueExpr); ok {
			hasKey = true
			break
		}
	}
	if !hasKey {
		et := check.geckoElemType(e.ElemList)
		typ := NewSlice(et)
		return typ, typ
	}
	var kt, vt Type
	for _, el := range e.ElemList {
		kv := el.(*syntax.KeyValueExpr)
		kt = check.geckoUnify(kt, check.geckoLitType(kv.Key))
		vt = check.geckoUnify(vt, check.geckoLitType(kv.Value))
	}
	typ := NewMap(kt, vt)
	return typ, typ
}

func (check *Checker) geckoElemType(es []syntax.Expr) Type {
	var et Type
	for _, e := range es {
		et = check.geckoUnify(et, check.geckoLitType(e))
	}
	return et
}

// geckoUnify folds two candidate types into one. Equal types are kept;
// anything else forces the `any` type. A nil type (the `null` literal)
// contributes nothing.
func (check *Checker) geckoUnify(a, b Type) Type {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if Identical(a, b) {
		return a
	}
	return universeAny.Type()
}

// geckoLitType reports the inferred type of a single dynamic literal element:
// literals map to their default types, true/false to bool, null to nil, and
// compound literals recursively to their inferred type. Any other expression
// yields `any`.
func (check *Checker) geckoLitType(e syntax.Expr) Type {
	switch e := syntax.Unparen(e).(type) {
	case *syntax.BasicLit:
		switch e.Kind {
		case syntax.IntLit:
			return Typ[Int]
		case syntax.FloatLit:
			return Typ[Float64]
		case syntax.ImagLit:
			return Typ[Complex128]
		case syntax.RuneLit:
			return Typ[Rune]
		case syntax.StringLit:
			return Typ[String]
		}
	case *syntax.Name:
		switch e.Value {
		case "true", "false":
			return Typ[Bool]
		case "null":
			return nil
		}
	case *syntax.CompositeLit:
		t, _ := check.geckoCompositeType(e)
		return t
	}
	return universeAny.Type()
}

// geckoBinaryNullish type-checks a nullish coalescing operation x ?? y,
// where the result is x unless x is null, in which case it is y. It
// reports the result in x. A bare null x yields y; a non-nilable x
// simply yields x. In all cases y must be assignable to the result
// type.
func (check *Checker) geckoBinaryNullish(x, y *operand, e syntax.Expr) {
	if x.isNil() {
		// null ?? y always selects the fallback: yield y.
		*x = *y
		x.expr = e
		return
	}

	// The result type is x's type, defaulted when x is untyped.
	typ := x.typ()
	if b, ok := typ.(*Basic); ok && b.info&IsUntyped != 0 {
		switch b.kind {
		case UntypedBool:
			typ = Typ[Bool]
		case UntypedInt:
			typ = Typ[Int]
		case UntypedRune:
			typ = Typ[Rune]
		case UntypedFloat:
			typ = Typ[Float64]
		case UntypedComplex:
			typ = Typ[Complex128]
		case UntypedString:
			typ = Typ[String]
		default:
			// UntypedNil cannot reach here (handled above); anything
			// else stays as is and is reported by the checks below.
		}
		x.typ_ = typ
	}

	if !isValid(typ) || !isValid(y.typ()) {
		x.invalidate()
		return
	}

	if y.isNil() {
		if !hasNil(typ) {
			check.errorf(y, MismatchedTypes, invalidOp+"cannot use null as the fallback of %s (type %s has no nil)", e, typ)
			x.invalidate()
		}
		return
	}

	if ok, _ := y.assignableTo(check, typ, nil); !ok {
		check.errorf(y, MismatchedTypes, invalidOp+"%s (mismatched types %s and %s)", e, y.typ(), typ)
		x.invalidate()
		return
	}
}

// GeckoTruthyKind classifies the type of a gecko conditional
// expression's condition, so that the noder knows which run-time test
// to build. Gecko's truthiness rules follow JavaScript: null, false,
// the numeric zero, the empty string and empty collections are falsy;
// every other value is truthy.
type GeckoTruthyKind int

const (
	// GeckoTruthyUnsupported is a type whose truthiness cannot be
	// determined at compile time, such as a type parameter.
	GeckoTruthyUnsupported GeckoTruthyKind = iota
	GeckoTruthyBool                        // the value itself
	GeckoTruthyNumber                      // x != 0
	GeckoTruthyLen                         // len(x) != 0
	GeckoTruthyNil                         // x != nil
	GeckoTruthyAlways                      // always truthy
	GeckoTruthyNever                       // always falsy
)

// GeckoTruthyKindOf returns how to test a value of type t for truthiness
// in a gecko conditional expression. It returns GeckoTruthyUnsupported if
// t has no such test.
func GeckoTruthyKindOf(t Type) GeckoTruthyKind {
	if t == nil {
		return GeckoTruthyUnsupported
	}
	switch u := Unalias(t).Underlying().(type) {
	case *Basic:
		switch {
		case isUntyped(u) && u.kind == UntypedNil:
			// A bare null is always falsy.
			return GeckoTruthyNever
		case isBoolean(u):
			return GeckoTruthyBool
		case isNumeric(u):
			return GeckoTruthyNumber
		case isString(u):
			return GeckoTruthyLen
		}
	case *Array:
		return GeckoTruthyLen
	case *Slice, *Map:
		return GeckoTruthyLen
	case *Pointer, *Chan, *Interface, *Signature:
		return GeckoTruthyNil
	case *Struct:
		return GeckoTruthyAlways
	}
	return GeckoTruthyUnsupported
}

// geckoConstTruthy reports whether the constant val is truthy under
// gecko's JavaScript truthiness rules: null, false, the numeric zero,
// and the empty string are falsy; every other constant is truthy.
func geckoConstTruthy(val constant.Value) bool {
	if val == nil {
		return false // invalid: report nothing, keep going
	}
	switch val.Kind() {
	case constant.Bool:
		return constant.BoolVal(val)
	case constant.String:
		return constant.StringVal(val) != ""
	}
	// Every numeric constant, including complex, is falsy only if it is
	// the zero value.
	return constant.Sign(val) != 0
}

// condExpr type-checks a gecko conditional expression
// cond ? then : else, reporting the result in x.
//
// The condition may be of any type: it is tested for truthiness under
// gecko's JavaScript rules. Only one branch is evaluated, but both are
// type-checked, and they must agree: the type of then is the result type
// and else must be assignable to it, or vice versa if then is a bare
// null. An untyped result keeps its untyped type so that a surrounding
// context can still default it, which is possible because the condition
// is a constant and the selected branch can be evaluated at compile
// time.
func (check *Checker) condExpr(x *operand, e *syntax.CondExpr) {
	// expr is the node as an Expr, which is how error messages render it.
	expr := syntax.Expr(e)
	var cond, then, els operand

	check.use(e.Cond, e.Then, e.Else)

	check.expr(nil, &cond, e.Cond)
	if !cond.isValid() {
		x.invalidate()
		return
	}
	check.expr(nil, &then, e.Then)
	if !then.isValid() {
		x.invalidate()
		return
	}
	check.expr(nil, &els, e.Else)
	if !els.isValid() {
		x.invalidate()
		return
	}

	// The branch that is not a bare null provides the result type.
	target, other := &then, &els
	if then.isNil() {
		target, other = &els, &then
	}
	if target.isNil() {
		// Both branches are a bare null, so there is no type to go by.
		check.errorf(&els, MismatchedTypes, invalidOp+"%s (both branches are null)", expr)
		x.invalidate()
		return
	}

	typ := target.typ()
	if !isValid(typ) {
		x.invalidate()
		return
	}

	// An untyped branch takes the type of the other branch, so a typed
	// branch determines the result type and the untyped branch is
	// converted to it.
	if isUntyped(typ) && !other.isNil() && !isUntyped(other.typ()) {
		target, other = other, target
		typ = target.typ()
	}

	// Both branches must produce a usable result. A bare null takes the
	// type of the other branch, so it is not defaulted or converted.
	// When the result type is untyped and the condition is not a
	// constant, the selected branch is not known at compile time, so the
	// untyped type is defaulted here instead.
	untyped := isUntyped(typ)
	constCond := cond.mode() == constant_

	if untyped && !constCond {
		// The selected branch is not known at compile time, so the
		// result is a value of the default type, not a constant.
		typ = Default(typ)
		untyped = false
	}

	// The branch that is not the result type must produce a result of
	// that type.
	if untyped {
		if !other.isNil() && !Identical(target.typ(), other.typ()) {
			check.errorf(other, MismatchedTypes, invalidOp+"%s (mismatched types %s and %s)", expr, target.typ(), other.typ())
			x.invalidate()
			return
		}
	} else {
		check.assignment(other, typ, "conditional expression")
		if !other.isValid() {
			x.invalidate()
			return
		}
	}

	// The result is the selected branch, which is known at compile time
	// when the condition is a constant. A selected bare null is a nil
	// value of the result type, not an untyped constant.
	if untyped {
		sel := target
		if !geckoConstTruthy(cond.val) {
			sel = other
		}
		if sel.isNil() {
			typ = Default(typ)
			x.mode_ = value
			x.typ_ = typ
			check.updateExprType(e.Then, typ, true)
			check.updateExprType(e.Else, typ, true)
			return
		}
		x.mode_ = constant_
		x.typ_ = target.typ()
		x.val = sel.val
		return
	}

	x.mode_ = value
	x.typ_ = typ

	// Settle the final type of the branches that were left untyped so that
	// the recorded types match what the noder will emit.
	check.updateExprType(e.Then, typ, true)
	check.updateExprType(e.Else, typ, true)

}

// isByteType reports whether t is byte (uint8) or a type with an
// underlying uint8 basic type.
func isByteType(t Type) bool {
	if t == nil || isTypeParam(t) {
		return false
	}
	b, ok := safeUnderlying(t).(*Basic)
	return ok && b.kind == Uint8
}

// isUntypedInteger reports whether t is an untyped integer constant
// type (untyped int, rune, or other untyped numeric types).
func isUntypedInteger(t Type) bool {
	return isUntypedNumeric(t) && isInteger(t)
}

// GeckoBinaryCommonType reports the common operand type that gecko's
// implicit byte conversions require for a gecko binary operation with
// operand types xt and yt. It returns nil if no gecko coercion applies,
// in which case the operation is handled by ordinary Go rules.
//
// Two coercions exist:
//
//   - byte + string (and string + byte) concatenate the byte as its
//     character: cur += c behaves as cur += string(c). An untyped integer
//     constant concatenated with a string likewise becomes its character.
//   - byte promotes to a wider numeric type in arithmetic and
//     comparisons: byte + byte → int, byte + int → int,
//     float64 + byte → float64, etc.
func GeckoBinaryCommonType(xt, yt Type, op syntax.Operator) Type {
	if op == syntax.Add && xString(xt) != xString(yt) {
		// Exactly one side is a string; the other concatenates as its
		// character when it is a byte or an untyped integer constant.
		var other Type
		if xString(xt) {
			other = yt
		} else {
			other = xt
		}
		if isByteType(other) || isUntypedInteger(other) {
			return Typ[String]
		}
		return nil
	}

	if isByteType(xt) || isByteType(yt) {
		if allNumeric(xt) && allNumeric(yt) {
			switch {
			case isByteType(xt) && isByteType(yt):
				return Typ[Int]
			case isByteType(xt) && isTyped(yt) && yt != Typ[Byte]:
				return yt
			case isTyped(xt) && xt != Typ[Byte]:
				return xt
			}
		}
	}
	return nil
}

// xString reports whether t is (or defaults to) a string type.
func xString(t Type) bool { return isString(t) }

// geckoCoerceBinary applies gecko's implicit byte conversions to the
// operands of a binary operation, in place, mutating their types (and,
// for an integer constant concatenated as a character, their value). It
// reports whether the operation was handled by gecko, in which case the
// operation now type-checks under ordinary Go rules.
func (check *Checker) geckoCoerceBinary(x, y *operand, op syntax.Operator, e, lhs, rhs syntax.Expr) bool {
	var pos syntax.Pos
	switch {
	case e != nil:
		pos = e.Pos()
	case lhs != nil:
		pos = lhs.Pos()
	case rhs != nil:
		pos = rhs.Pos()
	default:
		return false
	}
	if !check.geckoFile(pos) {
		return false
	}

	common := GeckoBinaryCommonType(x.typ(), y.typ(), op)
	if common == nil {
		return false
	}

	if op == syntax.Add && common == Typ[String] {
		// String concatenation: the non-string operand becomes its
		// character; integer constants are folded into a character string
		// so constant expressions keep working.
		toChar := func(z *operand) {
			if isString(z.typ()) {
				z.typ_ = Typ[String]
				return
			}
			if z.mode() == constant_ {
				if iv := constant.ToInt(z.val); iv.Kind() == constant.Int {
					if v, ok := constant.Int64Val(iv); ok {
						z.val = constant.MakeString(string(rune(v)))
						z.typ_ = Typ[String]
						return
					}
				}
				return // leave non-integer constants to be reported
			}
			z.typ_ = Typ[String]
		}
		toChar(x)
		toChar(y)
		return true
	}

	// Numeric promotion of byte.
	x.typ_ = common
	y.typ_ = common
	return true
}
