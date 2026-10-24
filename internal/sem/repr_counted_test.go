package sem_test

import (
	"testing"

	"kigumi/internal/sem"
)

const countedSrc = `
import {Box} from std/alloc

type Bag = { items: Array[String], cell: Box[Int] }
type Point = { x: Int, y: Int }
type Named = { at: Point, label: String }
type Held = Has(s: String) | Gone
type Res = resource { id: Int }
type Slot = { o: String? }
let p = Point { x: 1, y: 2 }
`

func TestCountedAdtTakesOptionAndResultOfReferences(t *testing.T) {
	res := checkRepr(t, countedSrc)
	tt := res.Types
	alloc := res.PackageMember(res.PackageByPath("std/prelude"), "AllocError")
	if alloc == 0 {
		t.Fatal("no AllocError in std/prelude")
	}
	allocErr := tt.Named(alloc, nil)
	bag := res.PackageMember(res.PackageByPath("main"), "Bag")
	strs, cell := fieldType(t, res, bag, "items"), fieldType(t, res, bag, "cell")
	for name, ty := range map[string]sem.TypeID{
		"String?":                      tt.Option(sem.TyString),
		"Bytes?":                       tt.Option(sem.TyBytes),
		"Result[String, AllocError]":   tt.Result(sem.TyString, allocErr),
		"Result[Int, String]":          tt.Result(sem.TyI64, sem.TyString),
		"Array[String]?":               tt.Option(strs),
		"Result[Box[Int], AllocError]": tt.Result(cell, allocErr),
		"Named?":                       tt.Option(mainType(t, res, "Named")),
		"Held?":                        tt.Option(mainType(t, res, "Held")),
		"String??":                     tt.Option(tt.Option(sem.TyString)),
		"Result[String?, AllocError]":  tt.Result(tt.Option(sem.TyString), allocErr),
	} {
		if !res.DirectAgg(ty) || !res.CountedAdt(ty) {
			t.Errorf("%s: direct %v, counted %v; want an inline value holding its references", name, res.DirectAgg(ty), res.CountedAdt(ty))
		}
	}
}

func TestCountedAdtLeavesOtherShapesAlone(t *testing.T) {
	res := checkRepr(t, countedSrc)
	tt := res.Types
	for name, ty := range map[string]sem.TypeID{
		"Int?":               tt.Option(sem.TyI64),
		"Point?":             tt.Option(mainType(t, res, "Point")),
		"Result[Int, Point]": tt.Result(sem.TyI64, mainType(t, res, "Point")),
	} {
		if !res.DirectAgg(ty) || res.CountedAdt(ty) {
			t.Errorf("%s: direct %v, counted %v; want a direct value with no counts", name, res.DirectAgg(ty), res.CountedAdt(ty))
		}
	}
	for name, ty := range map[string]sem.TypeID{
		"Held": mainType(t, res, "Held"),
		"Slot": mainType(t, res, "Slot"),
		"Res?": tt.Option(mainType(t, res, "Res")),
	} {
		if res.DirectAgg(ty) {
			t.Errorf("%s is direct; only Option and Result hold a reference inline, and never a resource", name)
		}
	}
	if res.CountFree(tt.Option(sem.TyString)) || !res.CountFree(tt.Option(sem.TyI64)) {
		t.Error("a counted value is not count-free, and a direct Int? is")
	}
}
