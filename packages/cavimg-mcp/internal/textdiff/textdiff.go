// Package textdiff produces a minimal single-hunk unified diff between two texts.
package textdiff

import (
	"fmt"
	"strings"
)

const context = 3

// Unified returns a unified diff between oldText and newText for the given path,
// or "" if they are identical.
func Unified(path, oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	a := splitLines(oldText)
	b := splitLines(newText)
	ops := diffLines(a, b)

	first, last := -1, -1
	for i, op := range ops {
		if op.tag != ' ' {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	if first == -1 {
		return "" // texts differ only by trailing newline; treat as no change
	}

	start := first - context
	if start < 0 {
		start = 0
	}
	end := last + context
	if end > len(ops)-1 {
		end = len(ops) - 1
	}

	// 1-based start lines = number of preceding lines present in each side + 1.
	aBefore, bBefore := 0, 0
	for _, op := range ops[:start] {
		if op.tag == ' ' || op.tag == '-' {
			aBefore++
		}
		if op.tag == ' ' || op.tag == '+' {
			bBefore++
		}
	}
	aCount, bCount := 0, 0
	var body strings.Builder
	for _, op := range ops[start : end+1] {
		switch op.tag {
		case ' ':
			aCount++
			bCount++
			body.WriteString(" " + op.text + "\n")
		case '-':
			aCount++
			body.WriteString("-" + op.text + "\n")
		case '+':
			bCount++
			body.WriteString("+" + op.text + "\n")
		}
	}
	aStart := aBefore + 1
	if aCount == 0 {
		aStart = aBefore
	}
	bStart := bBefore + 1
	if bCount == 0 {
		bStart = bBefore
	}

	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n", path)
	fmt.Fprintf(&out, "+++ b/%s\n", path)
	fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
	out.WriteString(body.String())
	return out.String()
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

type lineOp struct {
	tag  byte // ' ', '-', '+'
	text string
}

// diffLines computes a line-level diff via a longest-common-subsequence table.
func diffLines(a, b []string) []lineOp {
	n, m := len(a), len(b)
	c := make([][]int, n+1)
	for i := range c {
		c[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				c[i][j] = c[i-1][j-1] + 1
			} else if c[i-1][j] >= c[i][j-1] {
				c[i][j] = c[i-1][j]
			} else {
				c[i][j] = c[i][j-1]
			}
		}
	}
	var out []lineOp
	i, j := n, m
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			out = append(out, lineOp{' ', a[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || c[i][j-1] >= c[i-1][j]):
			out = append(out, lineOp{'+', b[j-1]})
			j--
		default:
			out = append(out, lineOp{'-', a[i-1]})
			i--
		}
	}
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return out
}
