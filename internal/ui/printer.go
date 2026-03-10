package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	cyan   = color.New(color.FgCyan)
	bold   = color.New(color.Bold)
	dim    = color.New(color.Faint)
)

func Success(msg string) {
	green.Fprintf(os.Stderr, "  ✓ %s\n", msg)
}

func Error(msg string) {
	red.Fprintf(os.Stderr, "  ✗ %s\n", msg)
}

func Warning(msg string) {
	yellow.Fprintf(os.Stderr, "  ! %s\n", msg)
}

func Info(msg string) {
	cyan.Fprintf(os.Stderr, "  → %s\n", msg)
}

func Header(msg string) {
	fmt.Fprintln(os.Stderr)
	bold.Fprintln(os.Stderr, msg)
}

func Dim(msg string) {
	dim.Fprintln(os.Stderr, msg)
}

func Line(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

func Linef(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func Table(rows [][]string) {
	if len(rows) == 0 {
		return
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	for _, row := range rows {
		parts := make([]string, len(row))
		for i, cell := range row {
			if i < len(widths) {
				parts[i] = cell + strings.Repeat(" ", widths[i]-len(cell))
			} else {
				parts[i] = cell
			}
		}
		fmt.Fprintf(os.Stderr, "  %s\n", strings.Join(parts, "  "))
	}
}

func ProgressWriter(total int64, label string) io.Writer {
	return &progressWriter{total: total, label: label}
}

type progressWriter struct {
	total   int64
	written int64
	label   string
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	if pw.total > 0 {
		pct := float64(pw.written) / float64(pw.total) * 100
		fmt.Fprintf(os.Stderr, "\r  → %s %.0f%%", pw.label, pct)
	} else {
		fmt.Fprintf(os.Stderr, "\r  → %s %.1f MB", pw.label, float64(pw.written)/1024/1024)
	}
	return n, nil
}
