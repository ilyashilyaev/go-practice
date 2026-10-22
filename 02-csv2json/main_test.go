package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseStudents(t *testing.T) {
	const header = "name,email,university,budget\n"

	tests := []struct {
		name     string
		input    string
		want     []Student
		wantWarn []string // подстроки, которые должны быть в предупреждениях
	}{
		{
			name:  "одна строка",
			input: header + "Анна,anna@example.com,МГУ,150000\n",
			want:  []Student{{Name: "Анна", Email: "anna@example.com", University: "МГУ", Budget: 150000}},
		},
		{
			name: "некорректный budget пропускается с номером строки",
			input: header +
				"Анна,anna@example.com,МГУ,150000\n" +
				"Борис,boris@example.com,СПбГУ,abc\n",
			want:     []Student{{Name: "Анна", Email: "anna@example.com", University: "МГУ", Budget: 150000}},
			wantWarn: []string{"line 3"},
		},
		{
			name:  "запятая и кавычки внутри поля",
			input: header + "\"Глеб \"\"Гек\"\" Орлов\",gleb@example.com,\"ВШЭ, Москва\",120000\n",
			want:  []Student{{Name: "Глеб \"Гек\" Орлов", Email: "gleb@example.com", University: "ВШЭ, Москва", Budget: 120000}},
		},
		{
			name:  "отрицательынй бюджет",
			input: header + "\"Фред \"\"Фред\"\" Зайцев\",fred@example.com,\"ВШЭ, Москва\",-1\n",
			want:  []Student{{Name: "Фред \"Фред\" Зайцев", Email: "fred@example.com", University: "ВШЭ, Москва", Budget: -1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var warn strings.Builder
			got, err := parseStudents(strings.NewReader(tt.input), &warn)
			if err != nil {
				t.Fatalf("parseStudents() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseStudents() = %+v, want %+v", got, tt.want)
			}
			for _, sub := range tt.wantWarn {
				if !strings.Contains(warn.String(), sub) {
					t.Errorf("warnings = %q, want substring %q", warn.String(), sub)
				}
			}
			if len(tt.wantWarn) == 0 && warn.Len() != 0 {
				t.Errorf("unexpected warnings: %q", warn.String())
			}
		})
	}
}

func TestGroupByUniversity(t *testing.T) {
	anna := Student{Name: "Анна", University: "МГУ"}
	boris := Student{Name: "Борис", University: "СПбГУ"}
	vera := Student{Name: "Вера", University: "МГУ"}

	got := groupByUniversity([]Student{anna, boris, vera})
	want := map[string][]Student{
		"МГУ":   {anna, vera},
		"СПбГУ": {boris},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groupByUniversity() = %+v, want %+v", got, want)
	}
}

func TestRun(t *testing.T) {
	const input = "name,email,university,budget\nАнна,anna@example.com,МГУ,150000\n"

	tests := []struct {
		name     string
		args     []string
		wantOut  string
		wantCode int
	}{
		{
			name:     "компактный JSON",
			wantOut:  `[{"name":"Анна","email":"anna@example.com","university":"МГУ","budget":150000}]` + "\n",
			wantCode: 0,
		},
		{
			name:     "группировка",
			args:     []string{"-group-by=university"},
			wantOut:  `{"МГУ":[{"name":"Анна","email":"anna@example.com","university":"МГУ","budget":150000}]}` + "\n",
			wantCode: 0,
		},
		{
			name: "pretty",
			args: []string{"-pretty"},
			wantOut: "[\n" +
				"  {\n" +
				"    \"name\": \"Анна\",\n" +
				"    \"email\": \"anna@example.com\",\n" +
				"    \"university\": \"МГУ\",\n" +
				"    \"budget\": 150000\n" +
				"  }\n" +
				"]\n",
			wantCode: 0,
		},
		{
			name:     "неизвестное поле группировки",
			args:     []string{"-group-by=email"},
			wantCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := run(tt.args, strings.NewReader(input), &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %q)", code, tt.wantCode, stderr.String())
			}
			if got := stdout.String(); got != tt.wantOut {
				t.Errorf("stdout = %q, want %q", got, tt.wantOut)
			}
		})
	}
}
