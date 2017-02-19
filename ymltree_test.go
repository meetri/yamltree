package ymltree

import (
	"testing"
)

var cfg1 = `
a: Easy!
b:
  c: 2
  d: [3, 4]
  e: 
    f: dog
    g: cake
`

var cfg2 = `
b:
  d: [4, 5, 6]
  e: 
    f: cat
    g: [1,2]
`

func TestMapSearch(t *testing.T) {
	cfg, _ := Load(cfg1)
	srch := cfg.Find("b/e/f")
	if srch != "dog" {
		t.Error("map search failed found", srch, "dog")
	}
}

func TestStringMapMerge(t *testing.T) {
	cfg, _ := Load(cfg1)
	cfg.Merge(cfg2)
	srch := cfg.Find("b/e/f")
	if srch != "cat" {
		t.Error("merge string override failed", srch, "cat")
	}
}

func TestTypeOverride(t *testing.T) {
	cfg, _ := Load(cfg1)
	cfg.Merge(cfg2)
	srch := cfg.Find("b/e/g")

	m := make(map[int]bool)
	for _, v := range srch.([]interface{}) {
		m[v.(int)] = true
	}

	expected := []int{1, 2}
	for _, v := range expected {
		if _, ok := m[v]; !ok {
			t.Error("missing expected element", v)
		}
	}
}

func TestIntSliceMapMerge(t *testing.T) {
	cfg, _ := Load(cfg1)
	cfg.Merge(cfg2)
	srch := cfg.Find("b/d")
	m := make(map[int]bool)
	for _, v := range srch.([]interface{}) {
		m[v.(int)] = true
	}

	if len(m) != 4 {
		t.Error("slice merge incorrect size", len(m), 4)
	}

	expected := []int{3, 4, 5, 6}
	for _, v := range expected {
		if _, ok := m[v]; !ok {
			t.Error("missing expected element", v)
		}
	}

}
