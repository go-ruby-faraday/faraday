// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import "testing"

func TestParamsBasics(t *testing.T) {
	p := NewParams()
	p.Set("a", "1")
	p.Set("b", "2")
	p.Set("a", "9") // overwrite keeps position
	if p.Len() != 2 {
		t.Fatalf("len = %d", p.Len())
	}
	if v, ok := p.Get("a"); !ok || v != "9" {
		t.Fatalf("a = %q,%v", v, ok)
	}
	if _, ok := p.Get("missing"); ok {
		t.Fatalf("missing should be absent")
	}
	if !p.Has("b") || p.Has("z") {
		t.Fatalf("Has broken")
	}
	if p.Pairs()[0].Key != "a" {
		t.Fatalf("order not preserved on overwrite")
	}
	if p.Encode() != "a=9&b=2" {
		t.Fatalf("Encode = %q", p.Encode())
	}
}

func TestParamsNilIndexSet(t *testing.T) {
	var p Params // zero value: nil index
	p.Set("k", "v")
	if v, _ := p.Get("k"); v != "v" {
		t.Fatalf("zero-value Set failed")
	}
}

func TestParamsDelete(t *testing.T) {
	p := ParamsOf([2]string{"a", "1"}, [2]string{"b", "2"}, [2]string{"c", "3"})
	p.Delete("z") // absent: no-op
	p.Delete("b") // present: removes and reindexes
	if p.Len() != 2 || p.Has("b") {
		t.Fatalf("delete failed: %v", p.Pairs())
	}
	if v, _ := p.Get("c"); v != "3" { // c still addressable after reindex
		t.Fatalf("c after delete = %q", v)
	}
}

func TestParamsCloneMerge(t *testing.T) {
	a := ParamsOf([2]string{"a", "1"}, [2]string{"b", "2"})
	c := a.Clone()
	c.Set("a", "changed")
	if v, _ := a.Get("a"); v != "1" {
		t.Fatalf("clone should be independent")
	}
	merged := a.Merge(ParamsOf([2]string{"b", "9"}, [2]string{"c", "3"}))
	if v, _ := merged.Get("b"); v != "9" {
		t.Fatalf("merge overwrite b = %q", v)
	}
	if v, _ := merged.Get("c"); v != "3" {
		t.Fatalf("merge new c = %q", v)
	}
	// Merge(nil) returns a plain clone.
	if a.Merge(nil).Len() != 2 {
		t.Fatalf("merge nil")
	}
}
