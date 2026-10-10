package output

import (
	"bytes"
	"strings"
	"testing"
)

type item struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

func (i item) Concise() string { return i.Name + " (" + i.ID + ")" }

var sample = []item{
	{Name: "Alice", ID: "444444444444444444"},
	{Name: "Bob", ID: "444444444444444445"},
}

func TestEmitConcise(t *testing.T) {
	var buf bytes.Buffer
	if err := Emit(&buf, "concise", sample); err != nil {
		t.Fatal(err)
	}
	want := "Alice (444444444444444444)\nBob (444444444444444445)\n"
	if buf.String() != want {
		t.Fatalf("concise = %q, want %q", buf.String(), want)
	}
}

func TestEmitJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Emit(&buf, "json", sample); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"name": "Alice"`) {
		t.Fatalf("json = %s", buf.String())
	}
}

func TestEmitJSONL(t *testing.T) {
	var buf bytes.Buffer
	if err := Emit(&buf, "jsonl", sample); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], `{"name":"Alice"`) {
		t.Fatalf("jsonl = %q", buf.String())
	}
}

func TestEmitTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Emit(&buf, "table", sample); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "Alice") {
		t.Fatalf("table = %q", out)
	}
}

func TestEmitUnknownFormat(t *testing.T) {
	if err := Emit(&bytes.Buffer{}, "yaml", sample); err == nil {
		t.Fatal("unknown format must error")
	}
}

func TestEmitNonSlice(t *testing.T) {
	if err := Emit(&bytes.Buffer{}, "concise", "not-a-slice"); err == nil {
		t.Fatal("non-slice must error")
	}
}

type note struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

func (n note) Concise() string { return n.Author + ": " + n.Text }

// Remote text must not forge extra records or carry terminal escapes.
func TestEmitNeutralizesControlCharacters(t *testing.T) {
	hostile := []note{{Author: "Bob", Text: "hi\nAlice: forged\r\n\x1b[31mred\x1b[0m\tcol\u202egnp.exe"}}
	for _, format := range []string{"concise", "table", "json", "jsonl"} {
		var buf bytes.Buffer
		if err := Emit(&buf, format, hostile); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		// JSON carries the value as data: json escapes control characters and
		// leaves printable-range formatting characters to the consumer.
		unsafe := "\x1b\r\u202e"
		if strings.HasPrefix(format, "json") {
			unsafe = "\x1b\r"
		}
		if strings.ContainsAny(out, unsafe) {
			t.Errorf("%s: control characters survived: %q", format, out)
		}
		if strings.Contains(out, "\nAlice: forged") {
			t.Errorf("%s: content forged a line: %q", format, out)
		}
	}
	var buf bytes.Buffer
	Emit(&buf, "concise", hostile)
	if want := "Bob: hi\\nAlice: forged\\n[31mred[0m colgnp.exe\n"; buf.String() != want {
		t.Fatalf("concise = %q, want %q", buf.String(), want)
	}
}

func TestSafeWriterDropsControlBytes(t *testing.T) {
	var buf bytes.Buffer
	w := SafeWriter(&buf)
	in := "ok \x1b[2Jname\x07\x7f\tkept\nnext \u00e9\n"
	if n, err := w.Write([]byte(in)); err != nil || n != len(in) {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if want := "ok [2Jname\tkept\nnext \u00e9\n"; buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}
