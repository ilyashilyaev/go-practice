// Command csv2json читает CSV со студентами и печатает их в формате JSON.
//
// Формат входа: заголовок name,email,university,budget и строки с данными.
// Файл передаётся аргументом; без аргумента читается stdin.
package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
)

// Student — одна строка CSV.
type Student struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	University string `json:"university"`
	Budget     int    `json:"budget"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run разбирает флаги, читает CSV, печатает JSON и возвращает код выхода.
// Как и в wc, работает с io.Reader/io.Writer, чтобы её можно было тестировать.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("csv2json", flag.ContinueOnError)
	fs.SetOutput(stderr)

	groupBy := fs.String("group-by", "", "группировать по полю (поддерживается: university)")
	pretty := fs.Bool("pretty", false, "форматированный вывод с отступами")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *groupBy != "" && *groupBy != "university" {
		warnf(stderr, "unknown -group-by %q", *groupBy)
		return 2
	}

	// Без аргумента читаем stdin, с аргументом — файл.
	// Тип in — io.Reader (выведен из stdin), поэтому в него можно положить и *os.File.
	in := stdin
	if fs.NArg() > 0 {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			warnf(stderr, "%v", err)
			return 1
		}
		defer func() { _ = f.Close() }()
		in = f
	}

	students, err := parseStudents(in, stderr)
	if err != nil {
		warnf(stderr, "%v", err)
		return 1
	}

	// writeJSON принимает any, поэтому одна переменная хранит и срез, и map.
	var out any = students
	if *groupBy == "university" {
		out = groupByUniversity(students)
	}

	if err := writeJSON(stdout, out, *pretty); err != nil {
		warnf(stderr, "write json: %v", err)
		return 1
	}

	return 0
}

// parseStudents читает CSV из r: первая строка — заголовок, остальные — данные.
// Строку с некорректным budget пропускает и пишет в warn предупреждение
// с номером строки в файле. Ошибку возвращает, только если CSV нельзя прочитать дальше.
func parseStudents(r io.Reader, warn io.Writer) ([]Student, error) {
	reader := csv.NewReader(r)
	students := make([]Student, 0)

	// Заголовок читаем и отбрасываем. Пустой вход — не ошибка, а пустой список.
	if _, err := reader.Read(); err != nil {
		if errors.Is(err, io.EOF) {
			return students, nil
		}
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv: %w", err)
		}

		budget, err := strconv.Atoi(record[3])
		if err != nil {
			// FieldPos даёт номер строки в файле, а не номер записи:
			// поле в кавычках может занимать несколько строк.
			line, _ := reader.FieldPos(3)
			warnf(warn, "line %d: invalid budget %q, row skipped", line, record[3])
			continue
		}

		students = append(students, Student{
			Name:       record[0],
			Email:      record[1],
			University: record[2],
			Budget:     budget,
		})
	}

	return students, nil
}

// groupByUniversity раскладывает студентов по университетам.
// Порядок студентов внутри группы — как во входе.
func groupByUniversity(students []Student) map[string][]Student {
	groups := make(map[string][]Student)
	for _, s := range students {
		// Для нового ключа groups[...] вернёт nil, а append(nil, s) создаст срез.
		groups[s.University] = append(groups[s.University], s)
	}
	return groups
}

// writeJSON кодирует v в JSON и пишет в w; при pretty — с отступом в два пробела.
func writeJSON(w io.Writer, v any, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// warnf печатает сообщение в stderr с префиксом "csv2json: ".
// Если не удалось записать даже в stderr, сообщить об этом уже некуда.
func warnf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "csv2json: "+format+"\n", args...)
}
