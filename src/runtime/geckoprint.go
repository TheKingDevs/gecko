// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"internal/abi"
	"internal/runtime/maps"
	"internal/strconv"
	"unsafe"
)

// geckoprint and geckoprintln implement the gecko print/println builtins
// whenever at least one argument has a custom representation (a slice or a
// map). All arguments are converted to interface{} and printed with
// geckoprintany; arguments are separated by a space and println appends a
// newline. The compiler emits these calls instead of the raw Go println
// builtin, which would print a slice like "[3/3]0x...".
func geckoprint(args ...any) {
	geckoprintall(args, false)
}

func geckoprintln(args ...any) {
	geckoprintall(args, true)
}

// geckotemplate concatenates the parts of a gecko template string used
// outside print/println. Each part is a literal segment or an interpolation,
// formatted exactly like a print/println argument by geckoAppendAny; a
// null-safe chain that triggered its nil guard arrives as a nil interface and
// renders as "null".
func geckotemplate(args ...any) string {
	var b []byte
	for _, arg := range args {
		b = geckoAppendAny(b, arg)
	}
	return string(b)
}

// geckoAppendAny appends the gecko formatting of v to b and returns the
// extended buffer. It mirrors the formatting in geckoprintany and
// geckoprintgeneric, but into a byte buffer instead of the console.
func geckoAppendAny(b []byte, v any) []byte {
	if v == nil {
		return append(b, "null"...)
	}
	switch v := v.(type) {
	case string:
		return append(b, v...)
	case bool:
		if v {
			return append(b, "true"...)
		}
		return append(b, "false"...)
	case int:
		return strconv.AppendInt(b, int64(v), 10)
	case int8:
		return strconv.AppendInt(b, int64(v), 10)
	case int16:
		return strconv.AppendInt(b, int64(v), 10)
	case int32:
		return strconv.AppendInt(b, int64(v), 10)
	case int64:
		return strconv.AppendInt(b, v, 10)
	case uint:
		return strconv.AppendUint(b, uint64(v), 10)
	case uint8:
		return strconv.AppendUint(b, uint64(v), 10)
	case uint16:
		return strconv.AppendUint(b, uint64(v), 10)
	case uint32:
		return strconv.AppendUint(b, uint64(v), 10)
	case uint64:
		return strconv.AppendUint(b, v, 10)
	case uintptr:
		return strconv.AppendUint(b, uint64(v), 10)
	case float32:
		return strconv.AppendFloat(b, float64(v), 'g', -1, 32)
	case float64:
		return strconv.AppendFloat(b, v, 'g', -1, 64)
	case complex64:
		return strconv.AppendComplex(b, complex128(v), 'g', -1, 64)
	case complex128:
		return strconv.AppendComplex(b, v, 'g', -1, 128)
	case []byte:
		b = geckoAppendSlice(b, "[", " ", "]", len(v), func(b []byte, i int) []byte {
			return strconv.AppendInt(b, int64(v[i]), 10)
		})
		return b
	case []int:
		b = geckoAppendSlice(b, "[", " ", "]", len(v), func(b []byte, i int) []byte {
			return strconv.AppendInt(b, int64(v[i]), 10)
		})
		return b
	case []int64:
		b = geckoAppendSlice(b, "[", " ", "]", len(v), func(b []byte, i int) []byte {
			return strconv.AppendInt(b, v[i], 10)
		})
		return b
	case []uint64:
		b = geckoAppendSlice(b, "[", " ", "]", len(v), func(b []byte, i int) []byte {
			return strconv.AppendUint(b, v[i], 10)
		})
		return b
	case []float64:
		b = geckoAppendSlice(b, "[", " ", "]", len(v), func(b []byte, i int) []byte {
			return strconv.AppendFloat(b, v[i], 'g', -1, 64)
		})
		return b
	case []bool:
		b = geckoAppendSlice(b, "[", " ", "]", len(v), func(b []byte, i int) []byte {
			if v[i] {
				return append(b, "true"...)
			}
			return append(b, "false"...)
		})
		return b
	case []string:
		b = geckoAppendSlice(b, "[", " ", "]", len(v), func(b []byte, i int) []byte {
			return append(b, v[i]...)
		})
		return b
	case []any:
		b = geckoAppendSlice(b, "[", " ", "]", len(v), func(b []byte, i int) []byte {
			return geckoAppendAny(b, v[i])
		})
		return b
	case map[string]any:
		sep := false
		b = append(b, '{')
		for k, e := range v {
			if sep {
				b = append(b, ", "...)
			}
			sep = true
			b = append(b, k...)
			b = append(b, ':')
			b = geckoAppendAny(b, e)
		}
		return append(b, '}')
	case map[string]string:
		sep := false
		b = append(b, '{')
		for k, e := range v {
			if sep {
				b = append(b, ", "...)
			}
			sep = true
			b = append(b, k...)
			b = append(b, ':')
			b = append(b, e...)
		}
		return append(b, '}')
	case map[string]int:
		sep := false
		b = append(b, '{')
		for k, e := range v {
			if sep {
				b = append(b, ", "...)
			}
			sep = true
			b = append(b, k...)
			b = append(b, ':')
			b = strconv.AppendInt(b, int64(e), 10)
		}
		return append(b, '}')
	case map[string]float64:
		sep := false
		b = append(b, '{')
		for k, e := range v {
			if sep {
				b = append(b, ", "...)
			}
			sep = true
			b = append(b, k...)
			b = append(b, ':')
			b = strconv.AppendFloat(b, e, 'g', -1, 64)
		}
		return append(b, '}')
	case map[string]bool:
		sep := false
		b = append(b, '{')
		for k, e := range v {
			if sep {
				b = append(b, ", "...)
			}
			sep = true
			b = append(b, k...)
			b = append(b, ':')
			if e {
				b = append(b, "true"...)
			} else {
				b = append(b, "false"...)
			}
		}
		return append(b, '}')
	default:
		return geckoAppendGeneric(b, v)
	}
}

// geckoAppendSlice appends `open elem0 sep elem1 ... close` to b.
func geckoAppendSlice(b []byte, open, sep, close string, n int, at func(b []byte, i int) []byte) []byte {
	b = append(b, open...)
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, sep...)
		}
		b = at(b, i)
	}
	return append(b, close...)
}

// geckoAppendGeneric formats any value not covered by the fast paths in
// geckoAppendAny, mirroring geckoprintgeneric: slices and maps print
// recursively, and anything else prints like the runtime printeface helper.
func geckoAppendGeneric(b []byte, v any) []byte {
	e := efaceOf(&v)
	if e._type == nil {
		return append(b, "null"...)
	}
	switch e._type.Kind() {
	case abi.Slice:
		return geckoAppendSliceGeneric(b, e._type, e.data)
	case abi.Map:
		return geckoAppendMapGeneric(b, e._type, e.data)
	default:
		b = append(b, '(')
		b = append(b, "0x"...)
		b = geckoAppendHex(b, uint64(uintptr(unsafe.Pointer(e._type))), 8)
		b = append(b, ',')
		b = append(b, "0x"...)
		b = geckoAppendHex(b, uint64(uintptr(e.data)), 8)
		return append(b, ')')
	}
}

// geckoAppendSliceGeneric appends a slice of elements of arbitrary type.
func geckoAppendSliceGeneric(b []byte, t *abi.Type, data unsafe.Pointer) []byte {
	sp := (*slice)(unsafe.Pointer(data))
	et := t.Elem()
	size := et.Size()
	b = append(b, '[')
	for i := 0; i < sp.len; i++ {
		if i > 0 {
			b = append(b, ' ')
		}
		p := unsafe.Pointer(uintptr(sp.array) + uintptr(i)*size)
		b = geckoAppendAny(b, geckoAny(et, p))
	}
	return append(b, ']')
}

// geckoAppendMapGeneric appends a map of arbitrary key/value types.
func geckoAppendMapGeneric(b []byte, t *abi.Type, data unsafe.Pointer) []byte {
	mt := (*abi.MapType)(unsafe.Pointer(t))
	elemType := mt.Elem
	keyType := mt.Key
	it := &maps.Iter{}
	it.Init(mt, (*maps.Map)(data))
	b = append(b, '{')
	first := true
	for {
		it.Next()
		key := it.Key()
		if key == nil {
			break
		}
		if !first {
			b = append(b, ", "...)
		}
		first = false
		b = geckoAppendAny(b, geckoAny(keyType, key))
		b = append(b, ':')
		b = geckoAppendAny(b, geckoAny(elemType, it.Elem()))
	}
	return append(b, '}')
}

// geckoAppendHex appends v as hexadecimal, padded with leading zeros to at
// least digits digits, mirroring the runtime printhex helper.
func geckoAppendHex(b []byte, v uint64, digits int) []byte {
	const dig = "0123456789abcdef"
	var buf [16]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = dig[v&0xf]
		v >>= 4
	}
	n := len(buf) - i
	if n < digits {
		n = digits
	}
	return append(b, buf[len(buf)-n:]...)
}

func geckoprintall(args []any, nl bool) {
	geckoprintbegin()
	for i, arg := range args {
		if i > 0 {
			printsp()
		}
		geckoprintany(arg)
	}
	// The trailing newline must be emitted before geckoprintend: output is
	// redirected to stdout only while the redirect is active, so printing it
	// afterwards would send it to stderr (and vanish when stdout is piped).
	if nl {
		printnl()
	}
	geckoprintend()
}

// geckoprintany prints a single value with the gecko print format:
// primitives look like the Go println builtin output, slices print as
// "[1 2 3]", maps as "{name:gecko, active:true}", and null as "null".
func geckoprintany(v any) {
	if v == nil {
		printstring("null")
		return
	}
	switch v := v.(type) {
	case string:
		printstring(v)
	case bool:
		printbool(v)
	case int:
		printint(int64(v))
	case int8:
		printint(int64(v))
	case int16:
		printint(int64(v))
	case int32:
		printint(int64(v))
	case int64:
		printint(v)
	case uint:
		printuint(uint64(v))
	case uint8:
		printuint(uint64(v))
	case uint16:
		printuint(uint64(v))
	case uint32:
		printuint(uint64(v))
	case uint64:
		printuint(v)
	case uintptr:
		printuint(uint64(v))
	case float32:
		printfloat32(v)
	case float64:
		printfloat64(v)
	case complex64:
		printcomplex64(v)
	case complex128:
		printcomplex128(v)
	case []byte:
		geckoprintslice("[", " ", "]", len(v), func(i int) { printint(int64(v[i])) })
	case []int:
		geckoprintslice("[", " ", "]", len(v), func(i int) { printint(int64(v[i])) })
	case []int64:
		geckoprintslice("[", " ", "]", len(v), func(i int) { printint(v[i]) })
	case []uint64:
		geckoprintslice("[", " ", "]", len(v), func(i int) { printuint(v[i]) })
	case []float64:
		geckoprintslice("[", " ", "]", len(v), func(i int) { printfloat64(v[i]) })
	case []bool:
		geckoprintslice("[", " ", "]", len(v), func(i int) { printbool(v[i]) })
	case []string:
		geckoprintslice("[", " ", "]", len(v), func(i int) { printstring(v[i]) })
	case []any:
		geckoprintslice("[", " ", "]", len(v), func(i int) { geckoprintany(v[i]) })
	case map[string]any:
		sep := geckoMapOpen()
		for k, e := range v {
			sep()
			printstring(k)
			printstring(":")
			geckoprintany(e)
		}
		geckoMapClose()
	case map[string]string:
		sep := geckoMapOpen()
		for k, e := range v {
			sep()
			printstring(k)
			printstring(":")
			printstring(e)
		}
		geckoMapClose()
	case map[string]int:
		sep := geckoMapOpen()
		for k, e := range v {
			sep()
			printstring(k)
			printstring(":")
			printint(int64(e))
		}
		geckoMapClose()
	case map[string]float64:
		sep := geckoMapOpen()
		for k, e := range v {
			sep()
			printstring(k)
			printstring(":")
			printfloat64(e)
		}
		geckoMapClose()
	case map[string]bool:
		sep := geckoMapOpen()
		for k, e := range v {
			sep()
			printstring(k)
			printstring(":")
			printbool(e)
		}
		geckoMapClose()
	default:
		geckoprintgeneric(v)
	}
}

// geckoprintgeneric prints any value that is not covered by the fast paths in
// geckoprintany. It looks at the runtime type of the value and, when it is a
// slice or map, prints it recursively so that arbitrarily nested structures
// (e.g. [][]int or map[int]string) print with the gecko format.
func geckoprintgeneric(v any) {
	e := efaceOf(&v)
	if e._type == nil {
		printstring("null")
		return
	}
	switch e._type.Kind() {
	case abi.Slice:
		geckoprintslicegeneric(e._type, e.data)
	case abi.Map:
		mt := (*abi.MapType)(unsafe.Pointer(e._type))
		elemType := mt.Elem
		keyType := mt.Key
		it := &maps.Iter{}
		it.Init(mt, (*maps.Map)(e.data))
		sep := geckoMapOpen()
		for {
			it.Next()
			key := it.Key()
			if key == nil {
				break
			}
			sep()
			geckoprintany(geckoAny(keyType, key))
			printstring(":")
			geckoprintany(geckoAny(elemType, it.Elem()))
		}
		geckoMapClose()
	default:
		printeface(*e)
	}
}

// geckoAny builds an interface value of type t whose data word points at p.
// For direct-iface types the value is stored in the interface itself, so the
// single word at p is copied.
func geckoAny(t *abi.Type, p unsafe.Pointer) any {
	var e eface
	e._type = t
	if t.TFlag&abi.TFlagDirectIface != 0 {
		e.data = *(*unsafe.Pointer)(p)
	} else {
		e.data = p
	}
	return *(*any)(unsafe.Pointer(&e))
}

// geckoprintslicegeneric prints a slice of elements of arbitrary type by
// walking the slice header and printing each element with geckoprintany.
func geckoprintslicegeneric(t *abi.Type, data unsafe.Pointer) {
	sp := (*slice)(unsafe.Pointer(data))
	et := t.Elem()
	size := et.Size()
	geckoprintslice("[", " ", "]", sp.len, func(i int) {
		p := unsafe.Pointer(uintptr(sp.array) + uintptr(i)*size)
		geckoprintany(geckoAny(et, p))
	})
}

var geckoMapFirst bool

// geckoMapOpen prints the opening `{` of a map and returns a function that
// must be called before printing each key/value pair (it prints a `, `
// separator after the first pair).
func geckoMapOpen() func() {
	printstring("{")
	geckoMapFirst = true
	return func() {
		if geckoMapFirst {
			geckoMapFirst = false
		} else {
			printstring(", ")
		}
	}
}

// geckoMapClose prints the closing `}` of a map.
func geckoMapClose() {
	printstring("}")
}

// geckoprintslice prints a slice as `open elem0 sep elem1 ... close`.
func geckoprintslice(open, sep, close string, n int, at func(i int)) {
	printstring(open)
	for i := 0; i < n; i++ {
		if i > 0 {
			printstring(sep)
		}
		at(i)
	}
	printstring(close)
}
