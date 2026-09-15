// run

// Copyright 2026 The Gecko Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test the gecko print, println, and input builtins. With an empty
// stdin (EOF), input(prompt) returns an empty string.

package main

func main() {
	print("P")
	println("Q")
	name := input("N")
	println("got", name, "len", len(name))
}
