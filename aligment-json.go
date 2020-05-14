package main

import "fmt"

// JSONEdit represents the JSON format of an edit
type JSONEdit struct {
	Type   string   `json:"type"`
	Source []string `json:"source,omitempty"`
	Target []string `json:"target,omitempty"`
}

// JSONEditer
type JSONEditer interface {
	ToJSONEdit() JSONEdit
}

// Explode creates new JSONEdits from the given one addressing them by contained IDS
func (e JSONEdit) Explode() (map[string]JSONEdit, map[string]JSONEdit) {
	e1 := map[string]JSONEdit{}
	e2 := map[string]JSONEdit{}

	for i, v := range e.Source {
		e1[v] = JSONEdit{Type: e.Type, Source: removeAt(i, e.Source), Target: copyArray(e.Target)}
	}
	for i, v := range e.Target {
		e2[v] = JSONEdit{Type: e.Type, Source: removeAt(i, e.Target), Target: copyArray(e.Source)}
	}
	return e1, e2
}

func removeAt(i int, a []string) []string {
	s := append([]string(nil), a[:i]...)
	return append(s, a[i+1:]...)
}

func copyArray(a []string) []string {
	c := make([]string, len(a))
	copy(c, a)
	return c
}

func (e *ins) ToJSONEdit() JSONEdit { return JSONEdit{Type: "ins", Target: []string{e.w.ID}} }
func (e *del) ToJSONEdit() JSONEdit { return JSONEdit{Type: "del", Source: []string{e.w.ID}} }
func (e *eq) ToJSONEdit() JSONEdit {
	return JSONEdit{Type: "eq", Source: []string{fmt.Sprint(e.from.ID)}, Target: []string{fmt.Sprint(e.to.ID)}}
}
func (e *sub) ToJSONEdit() JSONEdit {
	ss := make([]string, len(e.from))
	for i, v := range e.from {
		ss[i] = v.ID
	}
	tt := make([]string, len(e.to))
	for i, v := range e.to {
		tt[i] = v.ID
	}
	return JSONEdit{Type: "sub", Source: ss, Target: tt}
}

func (a *Alignment) ToJSONEdits() (map[string]JSONEdit, map[string]JSONEdit) {
	le := map[string]JSONEdit{}
	re := map[string]JSONEdit{}
	for _, v := range a.editMap {
		e := v.(JSONEditer).ToJSONEdit()
		e1, e2 := e.Explode()

		for k, ed := range e1 {
			if _, ok := le[k]; !ok {
				le[k] = ed
			}
		}
		for k, ed := range e2 {
			if _, ok := re[k]; !ok {
				re[k] = ed
			}
		}
	}
	return le, re
}

func MergeAlignments(a, b *Alignment) *Alignment {
	edits := make([]edit, len(a.editMap)+len(b.editMap))
	i := 0
	for _, v := range a.editMap {
		edits[i] = v
		i++
	}
	for _, v := range b.editMap {
		edits[i] = v
		i++
	}
	return newFromEdits(edits...)
}
