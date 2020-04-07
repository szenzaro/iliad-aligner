package main

import (
	"log"
	"testing"
)

func TestInclude(t *testing.T) {

	w := word{ID: "1", text: "asd"}

	a := newFromEdits(&ins{w: w})

	if !a.Includes(&ins{w: w}) {
		t.Error("fail")
	}
}

func TestEditAccuracy(t *testing.T) {
	wordsDB, err := loadDB("data/G44_I_III_HomPara.xlsx")
	if err != nil {
		log.Fatalln(err)
	}
	gs := loadGoldStandard("data/G44_ALI.tmx", wordsDB)

	// w := word{ID: "1", text: "asd"}
	// w2 := word{ID: "1", text: "asd"}

	var prob goldStandard

	for _, k := range gs {
		if k.ID == "2.265" {
			prob = k
		}
	}

	tt := []struct {
		a   *alignment
		b   *alignment
		acc float64
	}{
		// {a: newFromEdits(&ins{w: w}), b: newFromEdits(&ins{w: w2}), acc: 1.0},
		// {a: newFromEdits(&del{w: w}), b: newFromEdits(&del{w: w2}), acc: 1.0},
		// {a: newFromEdits(&ins{w: w}), b: newFromEdits(&del{w: w2}), acc: 0.0},
		// {a: newFromEdits(&eq{from: w, to: w}), b: newFromEdits(&eq{from: w2, to: w2}), acc: 1.0},
		// {a: newFromEdits(&sub{from: []word{w, w}, to: []word{w, w}}), b: newFromEdits(&sub{from: []word{w2, w2}, to: []word{w2, w2}}), acc: 1.0},
		{a: prob.a, b: prob.a, acc: 1.0},
	}

	for _, k := range tt {
		res := k.a.editsAccuracy(k.b)
		if res != k.acc {
			t.Error("Expected ", k.acc, " got ", res)
		}
	}
}
