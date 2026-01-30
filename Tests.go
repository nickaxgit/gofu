package main

import (
	"math"

	"github.com/nickax/gofu/log"
	"github.com/nickax/gofu/plane"
	"github.com/nickax/gofu/poly"
	"github.com/nickax/gofu/ray"
	"github.com/nickax/gofu/vec"
)

func testFloat(name string, f func() float64, expect float64, failMsg string) {

	v := f()
	if v != expect {
		log.Logit("FAILED", name, failMsg, "expected:", expect, "got:", v)
	} else {
		log.Logit("passed", name)
	}

}

func testBool(name string, f func() bool, expect bool, failMsg string) {

	v := f()
	if v != expect {
		log.Logit("FAILED", name, failMsg, "expected:", expect, "got:", v)
	} else {
		log.Logit("passed", name)
	}
}

func tests() {

	// kinks := []float64{1.0, 0.8, 1.0, 0.5, 0.25, 0.125, 1.0 / 16, 1.0 / 32, 1.0 / 64, 1.0 / 128, 1.0 / 256, 1.0 / 200, 1.0 / 300}

	// s := float64(100)
	// t1 := terrain.NewTriMesh("t1", 4, 1000, kinks, 100)
	// t1.AddVert(vec.NewVec3(0, 0, s), 0, 0)   //far
	// t1.AddVert(vec.NewVec3(s, 0, -s), 0, 0)  //right
	// t1.AddVert(vec.NewVec3(-s, 0, -s), 0, 0) //left

	// // t1.fi = []uint32{0, 1, 2}

	// tt := t1.triangleFrom(0)

	// testFloat("Triangle, distance from point/plane (negative)", func() float64 { return NewVec3(0, -50, 0).signedDistanceFromPlaneOf(tt) }, -50, "distanceFrom wrong")
	// testFloat("Triangle, distance from point/plane (positive)", func() float64 { return NewVec3(0, 50, 0).signedDistanceFromPlaneOf(tt) }, 50, "distanceFrom wrong")

	// testBool("Triangle contains, (exclude verts and edges) - point inside",
	// 	func() bool { return tt.contains(NewVec3(0, 0, 0), false, false) }, true, "contains wrong")
	// testBool("Triangle contains, (exclude verts and edges) - point outside", func() bool { return tt.contains(NewVec3(100, 0, 0), false, false) }, false, "contains wrong")
	// testBool("Triangle contains (point is vert) - include verts", func() bool { return tt.contains(NewVec3(0, 0, 100), true, true) }, true, "contains wrong")
	// testBool("Triangle contains (point is vert) - dont include verts", func() bool { return tt.contains(NewVec3(0, 0, 100), false, false) }, false, "contains wrong")

	// testBool("Triangle contains - on vertex - true", func() bool { return tt.contains(NewVec3(0, 0, 100), false, false) }, false, "contains (on vertex)wrong")
	// testBool("Triangle contains - on edge - true ", func() bool { return tt.contains(NewVec3(0, 0, 100), true, true) }, true, "contains (on vertex) wrong")

	// testFloat("Normal and edge are orthogonal ", func() float64 { return tt.normal.dot(tt.edge0()) }, 0, "normal and edge are not orthogonal")

	a := vec.NewVec3(0, 2, 0)
	b := vec.NewVec3(1.1, 0, 0)
	axis := vec.NewVec3(0, 0, 1)

	testFloat("Positive angle", func() float64 { return b.SignedAngleFrom(a, axis) }, math.Pi/2, "angle wrong")
	testFloat("Negative angle", func() float64 { return a.SignedAngleFrom(b, axis) }, -math.Pi/2, "angle wrong")

	testFloat("Cross product orthogonal", func() float64 { return a.Cross(b).Dot(a) }, 0, "cross product not orthogonal")

	p1 := plane.NewFromNormalAndPoint(vec.Up, vec.NewVec3(0, 0, 0))
	somePoint := vec.NewVec3(0, 10, 0)
	testFloat("Plane distance from point (on plane)", func() float64 { return p1.DistanceFrom(somePoint) }, 10, "plane (point on plane) distance wrong")

	testBool("Poly prob at vertex", func() bool {
		poly := poly.NewConvexPoly()
		poly.AddPointAt(0, -1, 10)
		poly.AddPointAt(10, 2, 0)
		poly.AddPointAt(-10, 3, 0)
		ray := ray.New(vec.NewVec3(-10, 1000, 0), vec.NewVec3(-10, -1000, 0)) //fire a vertical ray through a vertex
		hit, _ := poly.Probe(ray)
		return hit

	}, true, "poly at vertex probe failed")

	testBool("Poly probe outside ", func() bool {
		poly := poly.NewConvexPoly()
		poly.AddPointAt(0, -1, 10)
		poly.AddPointAt(10, 2, 0)
		poly.AddPointAt(-10, 3, 0)
		ray := ray.New(vec.NewVec3(-11, 1000, 0), vec.NewVec3(-10, -1000, 0))
		hit, _ := poly.Probe(ray)
		return hit

	}, false, "poly extended edge probe failed (should be outside)")

	testBool("Poly probe inside", func() bool {
		poly := poly.NewConvexPoly()
		poly.AddPointAt(0, -1, 10)
		poly.AddPointAt(10, 2, 0)
		poly.AddPointAt(-10, 3, 0)
		ray := ray.New(vec.NewVec3(5, 10000, 1), vec.NewVec3(3, -10000, 2))
		hit, _ := poly.Probe(ray)
		return hit
	}, true, "Poly probe inside")

	// New: probe the same polygon from "behind" (backfacing).
	// This ensures Poly.Probe does not implicitly backface-cull.
	testBool("Poly probe from behind (backfacing)", func() bool {
		poly := poly.NewConvexPoly()
		poly.AddPointAt(0, -1, 10)
		poly.AddPointAt(10, 2, 0)
		poly.AddPointAt(-10, 3, 0)

		// Use the same line as "Poly probe inside" but reversed.
		r := ray.New(vec.NewVec3(3, -10000, 2), vec.NewVec3(5, 10000, 1))
		hit, _ := poly.Probe(r)
		return hit
	}, true, "Poly probe from behind failed")

	testBool("Poly contains2D - 2", func() bool {
		ray := ray.New(vec.NewVec3(-14, 10000, 14), vec.NewVec3(-14, -10000, 14))
		poly := poly.NewConvexPoly()
		poly.AddPointAt(-20, 1762, 39)
		poly.AddPointAt(0, 1762, 0)
		poly.AddPointAt(-39, 1762, 0)
		hit, _ := poly.Probe(ray)
		return hit
	}, true, "Should be inside")

	//logit(len(facePens.pens))

}
