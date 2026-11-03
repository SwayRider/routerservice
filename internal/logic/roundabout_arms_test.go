package logic

import (
	"reflect"
	"testing"

	"github.com/swayrider/routerservice/internal/logic/logictest"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
)

func ie(heading int, drive, class string) vhtypes.TraceIntersectingEdge {
	return vhtypes.TraceIntersectingEdge{BeginHeading: heading, Driveability: drive, RoadClass: class}
}

func road(begin, end int) vhtypes.TraceEdge {
	return vhtypes.TraceEdge{BeginHeading: begin, EndHeading: end, RoadClass: "secondary"}
}

func ring(begin, end int, arms ...vhtypes.TraceIntersectingEdge) vhtypes.TraceEdge {
	e := vhtypes.TraceEdge{Roundabout: true, BeginHeading: begin, EndHeading: end}
	if len(arms) > 0 {
		e.EndNode = &vhtypes.TraceNode{IntersectingEdges: arms}
	}
	return e
}

// fourArms is a roundabout entered heading north, left by the west arm after
// passing the east and north arms (right-hand traffic, counter-clockwise).
func fourArms(extraEast ...vhtypes.TraceIntersectingEdge) []vhtypes.TraceEdge {
	east := append([]vhtypes.TraceIntersectingEdge{ie(90, "forward", "residential")}, extraEast...)
	return []vhtypes.TraceEdge{
		road(0, 0),
		ring(60, 30, east...),
		ring(10, 340, ie(0, "forward", "residential")),
		ring(340, 300),
		road(270, 270),
	}
}

func TestRoundaboutExits_FourArms(t *testing.T) {
	got, ok := RoundaboutExitsFromEdges(fourArms(), 3)
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{90, 0, 270}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestRoundaboutExits_EntryOnlyAndNotDrivableAreNotExits(t *testing.T) {
	// A one-way road into the roundabout and a cycleway: neither can be left by.
	edges := fourArms(ie(120, "backward", "tertiary"), ie(100, "", "service_other"))
	got, ok := RoundaboutExitsFromEdges(edges, 3)
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{90, 0, 270}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestRoundaboutExits_DrivewayCounts(t *testing.T) {
	// Valhalla numbers driveways and service roads like any other exit.
	got, ok := RoundaboutExitsFromEdges(fourArms(ie(120, "forward", "service_other")), 4)
	if !ok {
		t.Fatal("want ok")
	}
	if len(got.Angles) != 4 {
		t.Errorf("got %v, want 4 exits", got.Angles)
	}
}

func TestRoundaboutExits_InnerSideIsIgnored(t *testing.T) {
	// 330 leaves on the inner side of the path at the north node.
	edges := fourArms()
	edges[2].EndNode.IntersectingEdges = append(edges[2].EndNode.IntersectingEdges, ie(330, "forward", "residential"))
	if _, ok := RoundaboutExitsFromEdges(edges, 3); !ok {
		t.Error("an inner-side edge must not change the exit count")
	}
}

func TestRoundaboutExits_DisagreeingWithValhallaGivesNothing(t *testing.T) {
	for _, count := range []int{1, 2, 4} {
		if got, ok := RoundaboutExitsFromEdges(fourArms(), count); ok {
			t.Errorf("exit count %d: got %+v, want not ok", count, got)
		}
	}
}

func TestRoundaboutExits_LeftHandTrafficMirrors(t *testing.T) {
	got, ok := RoundaboutExitsFromEdges(mirrored(fourArms()), 3)
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{270, 0, 90}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func mirrored(edges []vhtypes.TraceEdge) []vhtypes.TraceEdge {
	mirror := func(h int) int { return (360 - h) % 360 }
	left := false
	for i := range edges {
		edges[i].BeginHeading = mirror(edges[i].BeginHeading)
		edges[i].EndHeading = mirror(edges[i].EndHeading)
		if edges[i].EndNode != nil {
			for j := range edges[i].EndNode.IntersectingEdges {
				edges[i].EndNode.IntersectingEdges[j].BeginHeading = mirror(edges[i].EndNode.IntersectingEdges[j].BeginHeading)
			}
		}
		edges[i].DriveOnRight = &left
	}
	return edges
}

func TestRoundaboutExits_NeedsEntryAndExitEdges(t *testing.T) {
	edges := fourArms()
	if _, ok := RoundaboutExitsFromEdges(edges[1:], 3); ok {
		t.Error("no edge before the roundabout: want not ok")
	}
	if _, ok := RoundaboutExitsFromEdges(edges[:4], 3); ok {
		t.Error("no edge after the roundabout: want not ok")
	}
	if _, ok := RoundaboutExitsFromEdges([]vhtypes.TraceEdge{road(0, 0), road(0, 0)}, 1); ok {
		t.Error("no roundabout: want not ok")
	}
}

// montgomery is a roundabout with split carriageways, reduced to headings and
// driveability: exit 5 of a roundabout entered heading 17.
func montgomery() []vhtypes.TraceEdge {
	return []vhtypes.TraceEdge{
		road(18, 18),
		road(19, 17),
		ring(76, 74, ie(332, "", "service_other")),
		ring(66, 66),
		ring(47, 47, ie(108, "forward", "tertiary")),
		ring(23, 15, ie(115, "backward", "tertiary")),
		ring(350, 350, ie(28, "forward", "secondary")),
		ring(325, 321, ie(70, "backward", "secondary")),
		ring(308, 308, ie(14, "forward", "secondary")),
		ring(294, 278, ie(21, "backward", "secondary")),
		ring(250, 245),
		ring(227, 227, ie(293, "forward", "residential")),
		ring(220, 220, ie(204, "forward", "secondary")),
		road(277, 282),
	}
}

func TestRoundaboutExits_SplitCarriageways(t *testing.T) {
	got, ok := RoundaboutExitsFromEdges(montgomery(), 5)
	if !ok {
		t.Fatal("want ok: the count must match Valhalla's 5th exit")
	}
	// The exit lanes leave at 108, 28, 14 and 293 degrees, and the exit taken at 277.
	if want := []int{91, 11, 357, 276, 260}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

// montgomeryLap is the roundabout of the user's route: 109, 50, 20 and 296
// degrees round from the road of entry (positions in the compass sense,
// relative to the heading on entry), then a tunnel at 290 right after the
// fourth, and the end of the lap. Every exit leaves 60 degrees off the
// tangent of the ring, so its heading says little about where it is.
func montgomeryLap(extra ...logictest.Node) ([]vhtypes.TraceEdge, []vhtypes.ShapePoint) {
	nodes := []logictest.Node{
		{Travelled: 71, Arms: []logictest.Arm{logictest.ExitArm(30)}},  // position 109
		{Travelled: 130, Arms: []logictest.Arm{logictest.ExitArm(30)}}, // position 50
		{Travelled: 160, Arms: []logictest.Arm{logictest.ExitArm(30)}}, // position 20
		{Travelled: 244, Arms: []logictest.Arm{logictest.ExitArm(30)}}, // position 296
		{Travelled: 250, Arms: []logictest.Arm{logictest.ExitArm(30)}}, // position 290: the tunnel
		{Travelled: 330},
	}
	return logictest.LapAround(49, false, append(nodes, extra...))
}

// headingsOf are the headings of the exits of montgomeryLap up to the nth,
// as the lookup over the route would give them.
func headingsOf(edges []vhtypes.TraceEdge, n int) RoundaboutExits {
	arms := outerExits(edges, 1, len(edges)-1)
	sortExits(arms, true)
	var known RoundaboutExits
	for _, a := range arms[:n] {
		known.Angles = append(known.Angles, a.angle)
	}
	return known
}

func TestFullRoundaboutExits_AnglesAreWhereTheExitsAreRoundTheRing(t *testing.T) {
	edges, shape := montgomeryLap()
	known := headingsOf(edges, 4)
	// The headings are far from the positions: 49 for the 109, 350 for the 50, ...
	if want := []int{49, 350, 320, 236}; !reflect.DeepEqual(known.Angles, want) {
		t.Fatalf("headings = %v, want %v", known.Angles, want)
	}

	got, ok := FullRoundaboutExits(edges, shape, known)
	if !ok {
		t.Fatal("want ok")
	}
	// The tunnel at 290 is 6 degrees from the exit taken at 296: not a road of its
	// own. Each angle is halfway between the position and the heading: the
	// 109 and 49 give 79, and so on.
	if want := []int{79, 20, 350, 266}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestFullRoundaboutExits_AnExitFarEnoughAfterOursIsKept(t *testing.T) {
	edges, shape := logictest.LapAround(49, false, []logictest.Node{
		{Travelled: 71, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 130, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 244, Arms: []logictest.Arm{logictest.ExitArm(30)}}, // position 296: taken
		{Travelled: 270, Arms: []logictest.Arm{logictest.ExitArm(30)}}, // position 270: 26 degrees on
		{Travelled: 330},
	})
	got, ok := FullRoundaboutExits(edges, shape, headingsOf(edges, 3))
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{79, 20, 266, 240}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestFullRoundaboutExits_ExitsUpToOursAreNeverLeftOut(t *testing.T) {
	// Two exits 10 degrees apart before ours: Valhalla counts both.
	edges, shape := logictest.LapAround(49, false, []logictest.Node{
		{Travelled: 80, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 90, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 200, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 330},
	})
	got, ok := FullRoundaboutExits(edges, shape, headingsOf(edges, 3))
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{70, 60, 310}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestFullRoundaboutExits_TheRoadWeCameFromIsNotAnExit(t *testing.T) {
	edges, shape := logictest.LapAround(49, false, []logictest.Node{
		{Travelled: 90, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 200},
		{Travelled: 300},
		{Travelled: 350, Arms: []logictest.Arm{logictest.ExitArm(30)}}, // back at the road of entry
	})
	got, ok := FullRoundaboutExits(edges, shape, headingsOf(edges, 1))
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{60}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestFullRoundaboutExits_LeftHandTrafficMirrors(t *testing.T) {
	edges, shape := logictest.LapAround(49, true, []logictest.Node{
		{Travelled: 71, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 130, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 244, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 250, Arms: []logictest.Arm{logictest.ExitArm(30)}},
		{Travelled: 330},
	})
	arms := outerExits(edges, 1, len(edges)-1)
	sortExits(arms, false)
	var known RoundaboutExits
	for _, a := range arms[:3] {
		known.Angles = append(known.Angles, a.angle)
	}
	got, ok := FullRoundaboutExits(edges, shape, known)
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{281, 340, 94}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestFullRoundaboutExits_MustAgreeWithTheRouteLookup(t *testing.T) {
	edges, shape := montgomeryLap()
	for name, known := range map[string][]int{
		"another first exit":      {80},
		"more exits than the lap": {49, 350, 320, 236, 230, 10, 20},
		"nothing known":           {},
	} {
		if got, ok := FullRoundaboutExits(edges, shape, RoundaboutExits{Angles: known}); ok {
			t.Errorf("%s: got %+v, want not ok", name, got)
		}
	}
}

func TestFullRoundaboutExits_NeedsAShapeThatFits(t *testing.T) {
	edges, shape := montgomeryLap()
	known := headingsOf(edges, 4)
	if _, ok := FullRoundaboutExits(edges, nil, known); ok {
		t.Error("no shape: want not ok")
	}
	if _, ok := FullRoundaboutExits(edges, shape[:6], known); ok {
		t.Error("shape too short for the indices: want not ok")
	}
	broken := append([]vhtypes.TraceEdge{}, edges...)
	broken[2].EndShapeIndex = -3
	if _, ok := FullRoundaboutExits(broken, shape, known); ok {
		t.Error("an index outside the shape: want not ok")
	}
	if _, ok := FullRoundaboutExits(edges, lapAroundShape(400), known); ok {
		t.Error("a shape that is not a roundabout's size: want not ok")
	}
}

// lapAroundShape is the shape of a lap of the given radius, as logictest.LapAround makes it.
func lapAroundShape(radius float64) []vhtypes.ShapePoint {
	_, shape := logictest.LapAround(radius, false, []logictest.Node{{Travelled: 71}, {Travelled: 130}, {Travelled: 160}, {Travelled: 244}, {Travelled: 250}, {Travelled: 330}})
	return shape
}

func TestFullRoundaboutExits_NeedsAWholeLap(t *testing.T) {
	edges, shape := montgomeryLap()
	if got, ok := FullRoundaboutExits(edges[:4], shape, headingsOf(edges, 2)); ok {
		t.Errorf("got %+v, want not ok: the walk stopped half way", got)
	}
	if _, ok := FullRoundaboutExits([]vhtypes.TraceEdge{road(0, 0), road(0, 0)}, shape, RoundaboutExits{Angles: []int{90}}); ok {
		t.Error("no roundabout: want not ok")
	}
}

func TestFullRoundaboutExits_NotDrivableAndEntryOnlyAreNotExits(t *testing.T) {
	edges, shape := logictest.LapAround(49, false, []logictest.Node{
		{Travelled: 71, Arms: []logictest.Arm{
			logictest.ExitArm(30),
			{Off: 20, Drive: "backward", Class: "tertiary"},
			{Off: 40, Drive: "", Class: "service_other"},
		}},
		{Travelled: 200},
		{Travelled: 330},
	})
	got, ok := FullRoundaboutExits(edges, shape, headingsOf(edges, 1))
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{79}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestHalfway(t *testing.T) {
	for _, c := range []struct{ a, b, want int }{
		{109, 49, 79},
		{50, 350, 20},   // across north
		{350, 50, 20},   // the other way round
		{10, 350, 0},    // straight on between them
		{90, 90, 90},    // the same
		{0, 180, 270},   // exactly opposite: 270 (the turn is -180)
		{300, 240, 270}, // west
	} {
		if got := halfway(c.a, c.b); got != c.want {
			t.Errorf("halfway(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
