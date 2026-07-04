// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import "testing"

func TestHeadersCaseInsensitive(t *testing.T) {
	h := HeadersOf([2]string{"Content-Type", "text/plain"})
	if v, ok := h.Get("content-type"); !ok || v != "text/plain" {
		t.Fatalf("case-insensitive get failed: %q,%v", v, ok)
	}
	h.Set("CONTENT-TYPE", "application/json") // overwrites, keeps first casing
	if h.Len() != 1 {
		t.Fatalf("len = %d, want 1", h.Len())
	}
	if h.Pairs()[0].Key != "Content-Type" {
		t.Fatalf("original casing not preserved: %q", h.Pairs()[0].Key)
	}
	if v, _ := h.Get("Content-Type"); v != "application/json" {
		t.Fatalf("value not updated: %q", v)
	}
	if !h.Has("content-TYPE") || h.Has("X-Nope") {
		t.Fatalf("Has broken")
	}
}

func TestHeadersSetDefault(t *testing.T) {
	h := NewHeaders()
	h.SetDefault("Accept", "a")
	h.SetDefault("accept", "b") // present: no clobber
	if v, _ := h.Get("Accept"); v != "a" {
		t.Fatalf("SetDefault clobbered: %q", v)
	}
	if _, ok := h.Get("Missing"); ok {
		t.Fatalf("absent get")
	}
}

func TestHeadersNilIndexSet(t *testing.T) {
	var h Headers
	h.Set("K", "v")
	if v, _ := h.Get("k"); v != "v" {
		t.Fatalf("zero-value Set failed")
	}
}

func TestHeadersDelete(t *testing.T) {
	h := HeadersOf([2]string{"A", "1"}, [2]string{"B", "2"}, [2]string{"C", "3"})
	h.Delete("z") // absent
	h.Delete("b") // case-insensitive present
	if h.Len() != 2 || h.Has("B") {
		t.Fatalf("delete failed")
	}
	if v, _ := h.Get("c"); v != "3" {
		t.Fatalf("reindex broke c: %q", v)
	}
}

func TestHeadersCloneMerge(t *testing.T) {
	a := HeadersOf([2]string{"A", "1"})
	c := a.Clone()
	c.Set("A", "2")
	if v, _ := a.Get("A"); v != "1" {
		t.Fatalf("clone independence")
	}
	m := a.Merge(HeadersOf([2]string{"a", "9"}, [2]string{"B", "3"}))
	if v, _ := m.Get("A"); v != "9" {
		t.Fatalf("merge overwrite")
	}
	if v, _ := m.Get("B"); v != "3" {
		t.Fatalf("merge new")
	}
	if a.Merge(nil).Len() != 1 {
		t.Fatalf("merge nil")
	}
}
