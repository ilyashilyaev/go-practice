package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  counts
	}{
		{name: "empty input", input: "", want: counts{}},
		{name: "one line", input: "hello world\n", want: counts{lines: 1, words: 2, bytes: 12}},
		{name: "one line", input: "hello world", want: counts{lines: 0, words: 2, bytes: 11}},
		{name: "one line", input: "hello world   ", want: counts{lines: 0, words: 2, bytes: 14}},
		{name: "one line", input: "hello world  парень ", want: counts{lines: 0, words: 3, bytes: 26}},
		{name: "more 32 KB", input: strings.Repeat("a\n", 20000), want: counts{lines: 20000, words: 20000, bytes: 40000}},
		{name: "3 long words", input: strings.Repeat("a", 70000) + " b c\n", want: counts{lines: 1, words: 3, bytes: 70005}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := count(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("count() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("count() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPrintCounts(t *testing.T) {
	small := counts{lines: 1, words: 2, bytes: 12}
	all := options{lines: true, words: true, bytes: true}

	tests := []struct {
		name  string
		opts  options
		fname string
		c     *counts // nil — используем small
		want  string
	}{
		{name: "all with file", opts: all, fname: "a.txt", want: "       1       2      12 a.txt\n"},
		{name: "all stdin", opts: all, fname: "", want: "       1       2      12\n"},
		{name: "lines and bytes", opts: options{lines: true, bytes: true}, fname: "a.txt", want: "       1      12 a.txt\n"},
		{name: "words only", opts: options{words: true}, fname: "", want: "       2\n"},
		{name: "total", opts: all, fname: "total", want: "       1       2      12 total\n"},
		{
			name:  "big numbers do not stick together",
			opts:  all,
			fname: "big.txt",
			c:     &counts{lines: 33554000, words: 134216000, bytes: 1073728000},
			want:  " 33554000 134216000 1073728000 big.txt\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := small
			if tt.c != nil {
				c = *tt.c
			}
			var sb strings.Builder
			if err := printCounts(&sb, c, tt.opts, tt.fname); err != nil {
				t.Fatalf("printCounts() error = %v", err)
			}
			if got := sb.String(); got != tt.want {
				t.Errorf("printCounts() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "nope.txt")

	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantOut    string
		wantErrSub string
		wantCode   int
	}{
		{
			name:     "stdin all counters",
			stdin:    "hello world\n",
			wantOut:  "       1       2      12\n",
			wantCode: 0,
		},
		{
			name:     "stdin -l",
			args:     []string{"-l"},
			stdin:    "a\nb\n",
			wantOut:  "       2\n",
			wantCode: 0,
		},
		{
			name:     "one file",
			args:     []string{a},
			wantOut:  "       1       2      12 " + a + "\n",
			wantCode: 0,
		},
		{
			name:     "flag order does not matter",
			args:     []string{"-c", "-l", a},
			wantOut:  "       1      12 " + a + "\n",
			wantCode: 0,
		},
		{
			name: "two files with total",
			args: []string{a, b},
			wantOut: "       1       2      12 " + a + "\n" +
				"       2       2       8 " + b + "\n" +
				"       3       4      20 total\n",
			wantCode: 0,
		},
		{
			name: "missing file continues",
			args: []string{missing, a},
			wantOut: "       1       2      12 " + a + "\n" +
				"       1       2      12 total\n",
			wantErrSub: "nope.txt",
			wantCode:   1,
		},
		{
			name:       "unknown flag",
			args:       []string{"-x"},
			wantErrSub: "flag provided but not defined",
			wantCode:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %q)", code, tt.wantCode, stderr.String())
			}
			if got := stdout.String(); got != tt.wantOut {
				t.Errorf("stdout = %q, want %q", got, tt.wantOut)
			}
			if tt.wantErrSub == "" && stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %q", stderr.String())
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr.String(), tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tt.wantErrSub)
			}
		})
	}
}

// countReference — прежняя реализация: всё в память, затем bytes.Count/bytes.Fields.
// Потоковый count обязан давать ровно тот же результат.
func countReference(data []byte) counts {
	return counts{
		lines: bytes.Count(data, []byte("\n")),
		words: len(bytes.Fields(data)),
		bytes: len(data),
	}
}

func TestCountStreaming(t *testing.T) {
	inputs := map[string]string{
		"ascii":               "hello world\nfoo  bar\tbaz\n",
		"cyrillic":            "привет мир\nкак дела\n",
		"unicode spaces":      "a b c　d\u0085e",
		"invalid utf8":        "\xff\xfe abc \xc3",
		"only spaces":         " \t\n\r\v\f ",
		"word across chunks":  strings.Repeat("a", bufSize-1) + "bc d\n",
		"rune across chunks":  strings.Repeat("a", bufSize-1) + "п р",
		"space across chunks": strings.Repeat("a", bufSize-1) + " b",
		"many chunks":         strings.Repeat("слово word\n", 10000),
	}

	// Разные Reader'ы отдают данные разными порциями — так ловим ошибки на границах.
	readers := map[string]func(string) io.Reader{
		"whole":    func(s string) io.Reader { return strings.NewReader(s) },
		"one byte": func(s string) io.Reader { return iotest.OneByteReader(strings.NewReader(s)) },
		"half":     func(s string) io.Reader { return iotest.HalfReader(strings.NewReader(s)) },
		"data+EOF": func(s string) io.Reader { return iotest.DataErrReader(strings.NewReader(s)) },
	}

	for inName, in := range inputs {
		want := countReference([]byte(in))
		for rName, mk := range readers {
			t.Run(inName+"/"+rName, func(t *testing.T) {
				got, err := count(mk(in))
				if err != nil {
					t.Fatalf("count() error = %v", err)
				}
				if got != want {
					t.Errorf("count() = %+v, want %+v", got, want)
				}
			})
		}
	}
}

func TestCountReadError(t *testing.T) {
	r := io.MultiReader(strings.NewReader("abc"), iotest.ErrReader(iotest.ErrTimeout))
	if _, err := count(r); !errors.Is(err, iotest.ErrTimeout) {
		t.Errorf("count() error = %v, want %v", err, iotest.ErrTimeout)
	}
}

// go test -bench=Count -benchmem
func BenchmarkCount(b *testing.B) {
	data := []byte(strings.Repeat("hello world привет мир\n", 1<<16)) // ~2 МБ
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := count(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}
