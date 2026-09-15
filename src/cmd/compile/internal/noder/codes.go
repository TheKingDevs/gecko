// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import "internal/pkgbits"

// A codeStmt distinguishes among statement encodings.
type codeStmt int

func (c codeStmt) Marker() pkgbits.SyncMarker { return pkgbits.SyncStmt1 }
func (c codeStmt) Value() int                 { return int(c) }

const (
	stmtEnd codeStmt = iota
	stmtLabel
	stmtBlock
	stmtExpr
	stmtSend
	stmtAssign
	stmtAssignOp
	stmtIncDec
	stmtBranch
	stmtCall
	stmtReturn
	stmtIf
	stmtFor
	stmtSwitch
	stmtSelect
)

// A codeExpr distinguishes among expression encodings.
type codeExpr int

func (c codeExpr) Marker() pkgbits.SyncMarker { return pkgbits.SyncExpr }
func (c codeExpr) Value() int                 { return int(c) }

// TODO(mdempsky): Split expr into addr, for lvalues.
const (
	exprConst  codeExpr = iota
	exprLocal           // local variable
	exprGlobal          // global variable or function
	exprCompLit
	exprFuncLit
	exprFieldVal
	exprMethodVal
	exprMethodExpr
	exprIndex
	exprSlice
	exprAssert
	exprUnaryOp
	exprBinaryOp
	exprCall
	exprConvert
	exprNew
	exprNewClass
	exprMake
	exprSizeof
	exprAlignof
	exprOffsetof
	exprZero
	exprFuncInst
	exprRecv
	exprReshape
	exprRuntimeBuiltin // a reference to a runtime function from transformed syntax. Followed by string name, e.g., "panicrangeexit"
	exprPrintTemplate  // gecko template string used as a print/println argument. Followed by bool println, parts for the flattened arguments
	exprGeckoPrint     // gecko print/println with values needing a custom form (slices, maps, null-safe chains). Followed by bool println, args as interface{}
	exprTemplate       // gecko template string used as a general string expression. Followed by parts: bool literal; string, or bool chain guards..., expr
)

type codeAssign int

func (c codeAssign) Marker() pkgbits.SyncMarker { return pkgbits.SyncAssign }
func (c codeAssign) Value() int                 { return int(c) }

const (
	assignBlank codeAssign = iota
	assignDef
	assignExpr
)

// A codeDecl distinguishes among declaration encodings.
type codeDecl int

func (c codeDecl) Marker() pkgbits.SyncMarker { return pkgbits.SyncDecl }
func (c codeDecl) Value() int                 { return int(c) }

const (
	declEnd codeDecl = iota
	declFunc
	declMethod
	declVar
	declOther
)
