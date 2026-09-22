package encoder

import (
	"encoding/json"
	"io"
	"testing"
)

func TestNativeQuoteControls(t *testing.T) {
	for i := 0; i < 32; i++ {
		s := string(byte(i))
		quoted := Quote(s)
		var out string
		if err := json.Unmarshal([]byte(quoted), &out); err != nil || out != s {
			t.Fatalf("byte %d: %q -> %q %v", i, quoted, out, err)
		}
	}
	if got := Quote("\xff"); got != "\"\xff\"" {
		t.Fatalf("native invalid UTF-8 preservation=%q", got)
	}
}

func TestNativeValidationPositions(t *testing.T) {
	cases := []struct {
		src   string
		valid bool
		pos   int
	}{
		{"", false, -1}, {" \n ", false, 3}, {`{"a":}`, false, 5}, {`[1,]`, false, 3},
		{`true false`, false, 5}, {`trX`, false, 2}, {`[falX]`, false, 4}, {`1e+`, false, 1},
		{`"\q"`, true, 0}, {`"\uZZZZ"`, true, 0}, {"\"x\ny\"", true, 0},
	}
	for _, tt := range cases {
		if ok, pos := Valid([]byte(tt.src)); ok != tt.valid || pos != tt.pos {
			t.Fatalf("Valid(%q)=%v,%d; want %v,%d", tt.src, ok, pos, tt.valid, tt.pos)
		}
	}
}

func TestEncodeIntoUsesExactCapacity(t *testing.T) {
	buf := make([]byte, 2, 5)
	copy(buf, "p:")
	base := &buf[0]
	if err := EncodeInto(&buf, 123, 0); err != nil || string(buf) != "p:123" || &buf[0] != base {
		t.Fatalf("append=%q err=%v reused=%v", buf, err, &buf[0] == base)
	}
	before := string(buf)
	if err := EncodeInto(&buf, make(chan int), 0); err == nil || string(buf) != before {
		t.Fatalf("failed encode changed destination: %q %v", buf, err)
	}
}

var benchmarkValue = struct {
	ID   int
	Name string
}{42, "sonic"}

func BenchmarkEncodeIntoReusedBuffer(b *testing.B) {
	buf := make([]byte, 0, 4096)
	b.ReportAllocs()
	for b.Loop() {
		buf = buf[:0]
		if err := EncodeInto(&buf, &benchmarkValue, 0); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkStreamEncoderReusedBuffer(b *testing.B) {
	e := NewStreamEncoder(io.Discard)
	b.ReportAllocs()
	for b.Loop() {
		if err := e.Encode(&benchmarkValue); err != nil {
			b.Fatal(err)
		}
	}
}
