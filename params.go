// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

// Pair is one entry of an ordered string-keyed map.
type Pair struct {
	Key string
	Val string
}

// Params is an insertion-ordered string→string map used for request query
// parameters and url-encoded bodies. Ruby's Faraday threads plain Hashes through
// the flow; this ordered map mirrors that, giving deterministic, order-preserving
// output for query strings (see [BuildQuery]).
type Params struct {
	pairs []Pair
	index map[string]int
}

// NewParams returns an empty ordered [Params].
func NewParams() *Params { return &Params{index: map[string]int{}} }

// ParamsOf builds a [Params] from ordered key/value pairs (later duplicates
// overwrite earlier values, keeping the first position — Ruby Hash semantics).
func ParamsOf(kv ...[2]string) *Params {
	p := NewParams()
	for _, e := range kv {
		p.Set(e[0], e[1])
	}
	return p
}

// Len reports the number of entries.
func (p *Params) Len() int { return len(p.pairs) }

// Pairs returns the entries in insertion order. The slice must not be mutated.
func (p *Params) Pairs() []Pair { return p.pairs }

// Set inserts or replaces the entry for key, preserving the position of an
// existing key when it is overwritten (last write wins on value, first write
// wins on order — the gem's Hash semantics).
func (p *Params) Set(key, val string) {
	if p.index == nil {
		p.index = map[string]int{}
	}
	if i, ok := p.index[key]; ok {
		p.pairs[i].Val = val
		return
	}
	p.index[key] = len(p.pairs)
	p.pairs = append(p.pairs, Pair{Key: key, Val: val})
}

// Get returns the value for key and whether it was present.
func (p *Params) Get(key string) (string, bool) {
	if i, ok := p.index[key]; ok {
		return p.pairs[i].Val, true
	}
	return "", false
}

// Has reports whether key is present.
func (p *Params) Has(key string) bool {
	_, ok := p.index[key]
	return ok
}

// Delete removes key if present, keeping the order of the remaining entries.
func (p *Params) Delete(key string) {
	i, ok := p.index[key]
	if !ok {
		return
	}
	p.pairs = append(p.pairs[:i], p.pairs[i+1:]...)
	delete(p.index, key)
	for j := i; j < len(p.pairs); j++ {
		p.index[p.pairs[j].Key] = j
	}
}

// Clone returns a shallow copy of p.
func (p *Params) Clone() *Params {
	c := NewParams()
	for _, e := range p.pairs {
		c.Set(e.Key, e.Val)
	}
	return c
}

// Merge returns a new [Params] with p's entries first (in order), overlaid by
// other's (an existing key is overwritten in place, a new key is appended). This
// is how a connection's default params merge with a request's, matching
// Faraday::Connection#build_url's params.merge(request.params).
func (p *Params) Merge(other *Params) *Params {
	out := p.Clone()
	if other != nil {
		for _, e := range other.pairs {
			out.Set(e.Key, e.Val)
		}
	}
	return out
}

// Encode renders the params as an order-preserving, escaped query string (see
// [BuildQuery]).
func (p *Params) Encode() string { return BuildQuery(p) }
