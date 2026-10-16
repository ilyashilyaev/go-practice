// Command wc считает строки, слова и байты в файлах или stdin.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// counts — результат подсчёта для одного входа.
type counts struct {
	lines, words, bytes int
}

// options — какие счётчики выводить.
type options struct {
	lines, words, bytes bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run разбирает флаги, обрабатывает входы и возвращает код выхода.
// Вместо os.Stdin/os.Stdout принимает интерфейсы — так её легко тестировать.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("wc", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts options
	fs.BoolVar(&opts.lines, "l", false, "check lines")
	fs.BoolVar(&opts.words, "w", false, "check words")
	fs.BoolVar(&opts.bytes, "c", false, "check bytes")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Ни одного флага — включаем все три, как настоящий wc.
	if opts == (options{}) {
		opts = options{lines: true, words: true, bytes: true}
	}

	// Файлов нет — читаем stdin; имя не печатаем.
	files := fs.Args()
	if len(files) == 0 {
		c, err := count(stdin)
		if err != nil {
			warnf(stderr, "stdin: %v", err)
			return 1
		}
		if err := printCounts(stdout, c, opts, ""); err != nil {
			warnf(stderr, "%v", err)
			return 1
		}
		return 0
	}

	exitCode := 0
	var total counts
	for _, name := range files {
		c, err := countFile(name)
		if err != nil {
			// Ошибка одного файла не останавливает остальные.
			warnf(stderr, "%v", err)
			exitCode = 1
			continue
		}
		total.lines += c.lines
		total.words += c.words
		total.bytes += c.bytes

		// Ошибка записи в stdout (например, закрытый пайп) — дальше писать бессмысленно.
		if err := printCounts(stdout, c, opts, name); err != nil {
			warnf(stderr, "%v", err)
			return 1
		}
	}

	if len(files) > 1 {
		if err := printCounts(stdout, total, opts, "total"); err != nil {
			warnf(stderr, "%v", err)
			return 1
		}
	}

	return exitCode
}

// countFile открывает файл, считает его и гарантированно закрывает.
func countFile(name string) (counts, error) {
	f, err := os.Open(name)
	if err != nil {
		return counts{}, err
	}
	// Файл открыт только на чтение: ошибка Close ничего не теряет, игнорируем явно.
	defer func() { _ = f.Close() }()

	return count(f)
}

// warnf печатает сообщение об ошибке в stderr с префиксом "wc: ".
// Если не удалось записать даже в stderr, сообщить об этом уже некуда.
func warnf(stderr io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(stderr, "wc: "+format+"\n", args...)
}

// bufSize — сколько байт читаем из r за один вызов Read.
const bufSize = 32 * 1024

// count читает r кусками по bufSize и возвращает счётчики.
// Память постоянная (~32 КБ) при любом размере входа.
//
// Слово — непрерывная последовательность не-пробельных символов,
// пробел определяется unicode.IsSpace (как в bytes.Fields).
func count(r io.Reader) (counts, error) {
	var c counts
	// Запас в utf8.UTFMax байт под хвост руны, перенесённый с прошлого куска.
	buf := make([]byte, bufSize+utf8.UTFMax)
	carry := 0      // сколько байт незаконченной руны лежит в начале buf
	inWord := false // находимся ли внутри слова (состояние между кусками)

	for {
		n, err := r.Read(buf[carry : carry+bufSize])
		c.bytes += n
		data := buf[:carry+n]
		// После ошибки (в том числе io.EOF) данных больше не будет:
		// неполную руну в конце уже нечем дополнить.
		final := err != nil

		i := 0
		for i < len(data) {
			var space bool
			if b := data[i]; b < utf8.RuneSelf {
				// Быстрый путь для ASCII: один байт = один символ.
				if b == '\n' {
					c.lines++
				}
				space = asciiSpace[b]
				i++
			} else {
				// Многобайтовая руна может быть разрезана границей куска.
				if !final && !utf8.FullRune(data[i:]) {
					break
				}
				ch, size := utf8.DecodeRune(data[i:])
				space = unicode.IsSpace(ch)
				i += size
			}

			if space {
				inWord = false
			} else if !inWord {
				inWord = true
				c.words++
			}
		}

		// Хвост неполной руны переносим в начало buf; он дочитается следующим Read.
		carry = copy(buf, data[i:])

		if err == io.EOF {
			return c, nil
		}
		if err != nil {
			return counts{}, err
		}
	}
}

// asciiSpace — пробельные ASCII-символы по unicode.IsSpace.
var asciiSpace = [utf8.RuneSelf]bool{'\t': true, '\n': true, '\v': true, '\f': true, '\r': true, ' ': true}

// printCounts печатает одну строку вывода wc: включённые счётчики и имя файла.
// opts приходит уже с применёнными дефолтами (см. парсинг флагов).
func printCounts(w io.Writer, c counts, opts options, name string) error {
	// Значения включённых колонок в фиксированном порядке:
	// lines -> words -> bytes, независимо от порядка флагов.
	var vals []int
	if opts.lines {
		vals = append(vals, c.lines)
	}
	if opts.words {
		vals = append(vals, c.words)
	}
	if opts.bytes {
		vals = append(vals, c.bytes)
	}

	// Как wc на macOS: пробел + число вправо в поле шириной 7.
	// Для чисел до 7 цифр это те же 8 символов, что и "%8d", но длинные
	// числа (большие файлы) не слипаются: "%8d" дал бы "335540001342160001".
	var sb strings.Builder
	for _, v := range vals {
		fmt.Fprintf(&sb, " %7d", v)
	}

	// Для stdin имя пустое — не печатаем ни имя, ни хвостовой пробел.
	if name != "" {
		sb.WriteByte(' ')
		sb.WriteString(name)
	}
	sb.WriteByte('\n')

	// Одна запись — одна проверка ошибки.
	_, err := io.WriteString(w, sb.String())
	return err
}
