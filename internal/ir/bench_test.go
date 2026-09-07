package ir

import (
	"os"
	"path/filepath"
	"testing"
)

func benchmarkLoad(b *testing.B, fixture string) {
	src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ir", fixture))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, diags, err := Load(src); err != nil || len(diags) > 0 {
			b.Fatalf("Load: %v %v", err, diags)
		}
	}
}

func BenchmarkLoadExample(b *testing.B)    { benchmarkLoad(b, "example.yaml") }
func BenchmarkLoadExampleMCP(b *testing.B) { benchmarkLoad(b, "example-mcp.yaml") }
