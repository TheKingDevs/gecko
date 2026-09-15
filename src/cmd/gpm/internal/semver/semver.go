// Package semver implements semantic-version comparison and npm-style
// version ranges for the gecko package manager.
package semver

import (
	"strings"

	"golang.org/x/mod/semver"
)

// normalize brings a bare semver string ("1.2.3") to the canonical module
// form expected by golang.org/x/mod/semver ("v1.2.3").
func normalize(v string) string {
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// Bare strips the leading "v" from a canonical module version.
func Bare(v string) string {
	if strings.HasPrefix(v, "v") {
		return v[1:]
	}
	return v
}

// Valid reports whether v is a valid semantic version.
func Valid(v string) bool { return semver.IsValid(normalize(v)) }

// Prerelease reports whether v carries a prerelease suffix ("-beta.1").
func Prerelease(v string) bool { return semver.Prerelease(normalize(v)) != "" }

// Compare orders two semantic versions; the result is -1, 0 or +1.
func Compare(a, b string) int { return semver.Compare(normalize(a), normalize(b)) }

// Max returns the higher of two versions.
func Max(a, b string) string {
	if Compare(a, b) >= 0 {
		return a
	}
	return b
}

// PickBest returns the highest version in versions that satisfies spec,
// preferring an exact/semantic match. spec may be "", "*", a dist tag, or a
// range such as "^1.2.0" / "~2.0" / ">=1.0.0 <2.0.0".
func PickBest(versions []string, spec, tagVal string) (string, bool) {
	if spec == "" || spec == "*" {
		if tagVal != "" && contains(versions, tagVal) {
			return tagVal, true
		}
		return maxOf(versions), true
	}
	if contains(versions, spec) {
		return spec, true
	}
	if tagVal != "" && spec == tagVal && contains(versions, tagVal) {
		return tagVal, true
	}
	rs, ok := parseRange(spec)
	if !ok {
		return "", false
	}
	allowPre := strings.Contains(spec, "-")
	for _, v := range sortedDesc(versions) {
		if !Valid(v) {
			continue
		}
		if Prerelease(v) && !allowPre {
			continue
		}
		if rs.match(v) {
			return v, true
		}
	}
	return "", false
}

func maxOf(versions []string) string {
	best := ""
	for _, v := range versions {
		if v == "" {
			continue
		}
		if best == "" || Compare(v, best) > 0 {
			best = v
		}
	}
	return best
}

func contains(versions []string, v string) bool {
	for _, x := range versions {
		if x == v {
			return true
		}
	}
	return false
}

func sortedDesc(versions []string) []string {
	out := append([]string(nil), versions...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && Compare(out[j-1], out[j]) < 0; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// npm-style ranges
// ---------------------------------------------------------------------------

type bound struct {
	ver      string
	included bool
}

type rset struct {
	branches [][2]bound
}

func (r rset) match(v string) bool {
	cv := normalize(v)
	for _, b := range r.branches {
		if b[0].ver != "" {
			cmp := semver.Compare(cv, b[0].ver)
			if cmp < 0 || (cmp == 0 && !b[0].included) {
				continue
			}
		}
		if b[1].ver != "" {
			cmp := semver.Compare(cv, b[1].ver)
			if cmp > 0 || (cmp == 0 && !b[1].included) {
				continue
			}
		}
		return true
	}
	return false
}

// parseRange parses a subset of node-semver ranges: exact, >, >=, <, <=, ^,
// ~, x-ranges ("1.2.x", "1", "1.2"), hyphen ranges, whitespace AND and "||".
func parseRange(s string) (rset, bool) {
	s = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), ""))
	if s == "" || s == "*" {
		return rset{branches: [][2]bound{{}}}, true
	}
	var out rset
	for _, alt := range strings.Split(s, "||") {
		b, ok := parseBranch(alt)
		if !ok {
			return rset{}, false
		}
		out.branches = append(out.branches, b)
	}
	return out, true
}

func parseBranch(s string) ([2]bound, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return [2]bound{}, true
	}
	if i := strings.Index(s, " - "); i >= 0 {
		return hyphenBounds(s[:i], s[i+3:]), true
	}
	var lo, hi bound
	for _, tok := range strings.Fields(s) {
		op := ""
		for _, pref := range []string{"^", "~", ">=", "<=", ">", "<", "="} {
			if strings.HasPrefix(tok, pref) {
				op = pref
				tok = tok[len(pref):]
				break
			}
		}
		if op != "" {
			tok = strings.TrimPrefix(strings.TrimPrefix(tok, "v"), "v")
		}
		switch op {
		case "^":
			l, h := caretBounds(tok)
			lo, hi = l, h
		case "~":
			l, h := tildeBounds(tok)
			lo, hi = l, h
		case ">":
			l, _ := expandPartial(tok, true)
			lo = bound{l.ver, false}
		case ">=":
			l, _ := expandPartial(tok, true)
			lo = l
		case "<":
			h, _ := expandPartial(tok, false)
			hi = bound{h.ver, false}
		case "<=":
			h, _ := expandPartial(tok, false)
			hi = h
		default: // "=" or bare
			l, h := expandPartial(tok, true)
			if l.ver == h.ver && !wild(tok) {
				lo, hi = l, h // exact version: both bounds inclusive
			} else {
				lo = l
				// Partial and wildcard upper bounds are exclusive, mirroring
				// node-semver ("1.2" -> <1.3.0, "1.x" -> <2.0.0).
				hi = bound{ver: h.ver, included: false}
			}
		}
	}
	return [2]bound{lo, hi}, true
}

func wild(s string) bool { return strings.ContainsAny(s, "xX*") || strings.Count(s, ".") < 2 }

func hyphenBounds(a, b string) [2]bound {
	lo, _ := expandPartial(a, true)
	hi, _ := expandPartial(strings.TrimPrefix(b, "v"), false)
	return [2]bound{lo, hi}
}

// expandPartial turns "1", "1.2", "1.2.x", "1.2.3" or "*" into bounds.
// keepLo is true when the version is the lower bound of a range.
func expandPartial(p string, keepLo bool) (bound, bound) {
	p = strings.ToLower(p)
	switch p {
	case "", "*", "x":
		return bound{}, bound{}
	}
	parts := strings.Split(p, ".")
	major, minor, patch := "0", "0", "0"
	if len(parts) > 0 && parts[0] != "x" && parts[0] != "*" {
		major = parts[0]
	}
	if len(parts) > 1 && parts[1] != "x" && parts[1] != "*" {
		minor = parts[1]
	}
	if len(parts) > 2 && parts[2] != "x" && parts[2] != "*" {
		patch = parts[2]
	}
	full := major + "." + minor + "." + patch
	switch {
	case len(parts) >= 3 && parts[2] != "x" && parts[2] != "*":
		return of(full), of(full)
	case len(parts) == 2 && parts[1] != "x" && parts[1] != "*":
		return of(full), of(next(major, minor, false))
	case len(parts) == 1 && parts[0] != "x" && parts[0] != "*":
		return of(full), of(next(major, "", false))
	case len(parts) >= 2 && (parts[1] == "x" || parts[1] == "*"):
		return of(full), of(next(major, "", true))
	default: // "1.2.x"
		return of(full), of(next(major, minor, false))
	}
}

func of(v string) bound { return bound{ver: "v" + v, included: true} }

func next(major, minor string, zero bool) string {
	if minor == "" {
		return nextMajor(major)
	}
	if zero {
		return nextMajor(major)
	}
	return major + "." + nextNum(minor) + ".0"
}

func nextMajor(major string) string { return nextNum(major) + ".0.0" }

func nextNum(s string) string {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return s
		}
		n = n*10 + int(c-'0')
	}
	return itoa(n + 1)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func caretBounds(p string) (bound, bound) {
	l, _ := expandPartial(p, true)
	if l.ver == "" {
		return l, l
	}
	v := strings.TrimPrefix(l.ver, "v")
	parts := strings.Split(v, ".")
	if parts[0] == "0" {
		if parts[1] != "0" {
			return l, excl("0." + nextNum(parts[1]) + ".0")
		}
		if parts[2] != "0" {
			return l, excl("0.0." + nextNum(parts[2]))
		}
		return l, excl("0.0.0")
	}
	return l, excl(nextNum(parts[0]) + ".0.0")
}

func tildeBounds(p string) (bound, bound) {
	l, _ := expandPartial(p, true)
	if l.ver == "" {
		return l, l
	}
	parts := strings.Split(strings.TrimPrefix(l.ver, "v"), ".")
	switch specifiedParts(p) {
	case 1: // "~1", "~1.x" -> <2.0.0
		return l, excl(nextNum(parts[0]) + ".0.0")
	default: // "~1.2", "~1.2.3", "~1.2.x" -> <1.3.0
		return l, excl(parts[0] + "." + nextNum(parts[1]) + ".0")
	}
}

// excl builds an exclusive (open) upper bound.
func excl(v string) bound { return bound{ver: "v" + v, included: false} }

// specifiedParts counts the leading version components that were explicitly
// written in p (trailing "x" / "*" / "" are not counted).
func specifiedParts(p string) int {
	parts := strings.Split(strings.ToLower(p), ".")
	n := len(parts)
	for n > 0 {
		switch parts[n-1] {
		case "x", "*", "":
			n--
			continue
		}
		return n
	}
	return n
}
