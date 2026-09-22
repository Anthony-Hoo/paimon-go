package rootbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
	"io"
	"strings"
	"testing"
)

var sinkNode ast.Node
var workloadValue = struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}{42, "paimon", true}

func BenchmarkContainer(b *testing.B) {
	for _, n := range []int{10, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := `{"items":[` + strings.TrimSuffix(strings.Repeat(`{"id":42,"name":"paimon","ok":true},`, n), ",") + `]}`
			data := []byte(src)
			b.Run("Get", func(b *testing.B) {
				for b.Loop() {
					v, err := sonic.Get(data, "items")
					if err != nil {
						b.Fatal(err)
					}
					sinkNode = v
				}
			})
			b.Run("SearcherDefault", func(b *testing.B) {
				s := ast.NewSearcher(src)
				for b.Loop() {
					v, err := s.GetByPath("items")
					if err != nil {
						b.Fatal(err)
					}
					sinkNode = v
				}
			})
			b.Run("SearcherNoValidate", func(b *testing.B) {
				s := ast.NewSearcher(src)
				s.ValidateJSON = false
				for b.Loop() {
					v, err := s.GetByPath("items")
					if err != nil {
						b.Fatal(err)
					}
					sinkNode = v
				}
			})
			b.Run("LoadAll", func(b *testing.B) {
				for b.Loop() {
					v := ast.NewRaw(src)
					if err := v.LoadAll(); err != nil {
						b.Fatal(err)
					}
					sinkNode = v
				}
			})
		})
	}
}
func BenchmarkStringGet(b *testing.B) {
	for _, n := range []int{16, 65536} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := `{"id":42,"padding":"` + strings.Repeat("x", n) + `"}`
			data := []byte(src)
			b.Run("Bytes", func(b *testing.B) {
				for b.Loop() {
					v, err := sonic.Get(data, "id")
					if err != nil {
						b.Fatal(err)
					}
					sinkNode = v
				}
			})
			b.Run("String", func(b *testing.B) {
				for b.Loop() {
					v, err := sonic.GetFromString(src, "id")
					if err != nil {
						b.Fatal(err)
					}
					sinkNode = v
				}
			})
			b.Run("Searcher", func(b *testing.B) {
				for b.Loop() {
					v, err := ast.NewSearcher(src).GetByPath("id")
					if err != nil {
						b.Fatal(err)
					}
					sinkNode = v
				}
			})
			b.Run("CopyString", func(b *testing.B) {
				for b.Loop() {
					v, err := sonic.GetCopyFromString(src, "id")
					if err != nil {
						b.Fatal(err)
					}
					sinkNode = v
				}
			})
		})
	}
}
func BenchmarkEncoder(b *testing.B) {
	b.Run("RootStream", func(b *testing.B) {
		e := sonic.ConfigDefault.NewEncoder(io.Discard)
		for b.Loop() {
			if err := e.Encode(&workloadValue); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("StdStream", func(b *testing.B) {
		e := json.NewEncoder(io.Discard)
		e.SetEscapeHTML(false)
		for b.Loop() {
			if err := e.Encode(&workloadValue); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("RootMarshal", func(b *testing.B) {
		for b.Loop() {
			var err error
			sinkBytes, err = sonic.Marshal(&workloadValue)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("EncodeIntoReuse", func(b *testing.B) {
		out := make([]byte, 0, 1024)
		for b.Loop() {
			out = out[:0]
			if err := encoder.EncodeInto(&out, &workloadValue, 0); err != nil {
				b.Fatal(err)
			}
			sinkBytes = out
		}
	})
}
func BenchmarkDecodeNumbers(b *testing.B) {
	src := []byte(`{"a":1,"b":2,"c":3,"d":[4,5,6]}`)
	for _, mode := range []string{"Default", "UseNumber", "UseInt64"} {
		b.Run(mode, func(b *testing.B) {
			api := sonic.Config{UseNumber: mode == "UseNumber", UseInt64: mode == "UseInt64"}.Froze()
			for b.Loop() {
				var v any
				if err := api.Unmarshal(src, &v); err != nil {
					b.Fatal(err)
				}
				sinkAny = v
			}
		})
	}
}
func BenchmarkStringDecoder(b *testing.B) {
	src := strings.TrimSpace(strings.Repeat(`{"id":42} `, 100))
	b.Run("PackageDecoder", func(b *testing.B) {
		for b.Loop() {
			d := decoder.NewDecoder(src)
			for i := 0; i < 100; i++ {
				var v struct{ ID int }
				if err := d.Decode(&v); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("StdDecoder", func(b *testing.B) {
		for b.Loop() {
			d := json.NewDecoder(strings.NewReader(src))
			for i := 0; i < 100; i++ {
				var v struct{ ID int }
				if err := d.Decode(&v); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}
func TestBasicEquivalence(t *testing.T) {
	var a, b bytes.Buffer
	e := sonic.ConfigDefault.NewEncoder(&a)
	if err := e.Encode(&workloadValue); err != nil {
		t.Fatal(err)
	}
	s := json.NewEncoder(&b)
	s.SetEscapeHTML(false)
	if err := s.Encode(&workloadValue); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal(a.String(), b.String())
	}
}

type noopVisitor struct{}

func (noopVisitor) OnNull() error                        { return nil }
func (noopVisitor) OnBool(bool) error                    { return nil }
func (noopVisitor) OnString(string) error                { return nil }
func (noopVisitor) OnInt64(int64, json.Number) error     { return nil }
func (noopVisitor) OnFloat64(float64, json.Number) error { return nil }
func (noopVisitor) OnObjectBegin(int) error              { return nil }
func (noopVisitor) OnObjectKey(string) error             { return nil }
func (noopVisitor) OnObjectEnd() error                   { return nil }
func (noopVisitor) OnArrayBegin(int) error               { return nil }
func (noopVisitor) OnArrayEnd() error                    { return nil }
func BenchmarkPreorderDepth(b *testing.B) {
	for _, n := range []int{64, 256, 1024} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := strings.Repeat("[", n) + "0" + strings.Repeat("]", n)
			for b.Loop() {
				if err := ast.Preorder(src, noopVisitor{}, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkValidStringLarge(b *testing.B) {
	src := `{"payload":"` + strings.Repeat("x", 65536) + `"}`
	b.ReportAllocs()
	for b.Loop() {
		sinkBool = sonic.ValidString(src)
	}
}

func BenchmarkUnmarshalLargeString(b *testing.B) {
	data := []byte(`{"payload":"` + strings.Repeat("x", 65536) + `"}`)
	for b.Loop() {
		var out map[string]any
		if err := sonic.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
		sinkAny = out
	}
}
