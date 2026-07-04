// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import "strings"

// Headers is a case-insensitive, insertion-ordered header map, mirroring
// Faraday::Utils::Headers. A lookup matches keys case-insensitively while the
// original casing of the first-seen key is preserved for iteration and display
// (so "Content-Type" and "content-type" address the same entry).
type Headers struct {
	pairs []Pair
	index map[string]int // lower-cased key -> position in pairs
}

// NewHeaders returns an empty [Headers].
func NewHeaders() *Headers { return &Headers{index: map[string]int{}} }

// HeadersOf builds a [Headers] from ordered key/value pairs.
func HeadersOf(kv ...[2]string) *Headers {
	h := NewHeaders()
	for _, e := range kv {
		h.Set(e[0], e[1])
	}
	return h
}

// Len reports the number of headers.
func (h *Headers) Len() int { return len(h.pairs) }

// Pairs returns the headers in insertion order (with first-seen key casing). The
// slice must not be mutated.
func (h *Headers) Pairs() []Pair { return h.pairs }

// Set inserts or replaces the value for key (case-insensitive). An existing key
// keeps its original casing and position; only the value is updated.
func (h *Headers) Set(key, val string) {
	if h.index == nil {
		h.index = map[string]int{}
	}
	lk := strings.ToLower(key)
	if i, ok := h.index[lk]; ok {
		h.pairs[i].Val = val
		return
	}
	h.index[lk] = len(h.pairs)
	h.pairs = append(h.pairs, Pair{Key: key, Val: val})
}

// SetDefault inserts key→val only if key is absent (case-insensitive), matching
// middleware that fills in a header without clobbering a caller override.
func (h *Headers) SetDefault(key, val string) {
	if !h.Has(key) {
		h.Set(key, val)
	}
}

// Get returns the value for key (case-insensitive) and whether it was present.
func (h *Headers) Get(key string) (string, bool) {
	if i, ok := h.index[strings.ToLower(key)]; ok {
		return h.pairs[i].Val, true
	}
	return "", false
}

// Has reports whether key is present (case-insensitive).
func (h *Headers) Has(key string) bool {
	_, ok := h.index[strings.ToLower(key)]
	return ok
}

// Delete removes key (case-insensitive) if present, keeping the order of the
// remaining headers.
func (h *Headers) Delete(key string) {
	lk := strings.ToLower(key)
	i, ok := h.index[lk]
	if !ok {
		return
	}
	h.pairs = append(h.pairs[:i], h.pairs[i+1:]...)
	delete(h.index, lk)
	for j := i; j < len(h.pairs); j++ {
		h.index[strings.ToLower(h.pairs[j].Key)] = j
	}
}

// Clone returns a shallow copy of h.
func (h *Headers) Clone() *Headers {
	c := NewHeaders()
	for _, e := range h.pairs {
		c.Set(e.Key, e.Val)
	}
	return c
}

// Merge overlays other's headers onto a copy of h and returns the result (an
// existing key is overwritten in place, a new key is appended).
func (h *Headers) Merge(other *Headers) *Headers {
	out := h.Clone()
	if other != nil {
		for _, e := range other.pairs {
			out.Set(e.Key, e.Val)
		}
	}
	return out
}
