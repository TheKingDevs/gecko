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
