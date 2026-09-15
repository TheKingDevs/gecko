// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"unsafe"
)

// geckoinput implements the input builtin. It formats the arguments like
// println (joined with spaces, no trailing newline) as the prompt, writes
// it to stdout (fd 1), then drains stdin (fd 0) until EOF and returns the
// entire accumulated content. Piped input is therefore consumed fully in
// a single call; interactive users signal the end of input with Ctrl-D.
// Windows-style CRLF line endings are normalized to LF.
//
// Reads go through geckoinputBuf, which also serves as the sole staging
// area for stdin, so arbitrarily long input is assembled across reads.
func geckoinput(args ...any) string {
	var ps []byte
	for i, a := range args {
		if i != 0 {
			ps = append(ps, ' ')
		}
		ps = geckoAppendAny(ps, a)
	}
	if len(ps) != 0 {
		write(1, unsafe.Pointer(&ps[0]), int32(len(ps)))
	}

	var all []byte
	for {
		// Empty the retained buffer first, then read more from stdin.
		chunk := geckoinputBuf[geckoinputPos:geckoinputLen]
		
		// Check if there is a newline in the current chunk
		newlinePos := -1
		for i, b := range chunk {
			if b == '\n' {
				newlinePos = i
				break
			}
		}

		if newlinePos != -1 {
			// Found newline: consume up to that point
			all = append(all, chunk[:newlinePos+1]...)
			geckoinputPos += newlinePos + 1
			break
		}

		// No newline: consume everything
		all = append(all, chunk...)
		geckoinputPos = geckoinputLen
		if geckoinputPos == len(geckoinputBuf) {
			// The buffer is full; wrap around to the start for the next read.
			geckoinputPos, geckoinputLen = 0, 0
		}
		
		n := read(fdStdin, noescape(unsafe.Pointer(&geckoinputBuf[geckoinputPos])), int32(len(geckoinputBuf)-geckoinputPos))
		if n < 0 {
			if n == -int32(eintr) {
				continue // interrupted system call; retry
			}
			break // read error
		}
		if n == 0 {
			break // EOF: all input has been drained
		}
		geckoinputLen = geckoinputPos + int(n)
	}

	// Normalize CRLF to LF (a lone CR is dropped along with the Windows
	// line-ending CR).
	out := all[:0]
	for _, b := range all {
		if b != '\r' {
			out = append(out, b)
		}
	}
	return string(out)
}

const inputBufLen = 512

// geckoinputBuf stages stdin reads. geckoinputPos is the next unconsumed
// byte and geckoinputLen the number of valid bytes.
var (
	geckoinputBuf [inputBufLen]byte
	geckoinputPos = 0
	geckoinputLen = 0
)

const (
	fdStdin = 0 // read end of input
	// eintr is the errno value for an interrupted system call on Unix.
	// It is only used to retry reads that fail with EINTR.
	eintr = 4
)
