package logic

import (
	"reflect"
	"testing"

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

// lap is a full lap round the roundabout of fourArms, entered heading north:
// the edges go on past the west exit and round to the road the roundabout
// was entered from.
func lap(extra ...vhtypes.TraceIntersectingEdge) []vhtypes.TraceEdge {
	return []vhtypes.TraceEdge{
		road(0, 0),
		ring(80, 50, append([]vhtypes.TraceIntersectingEdge{ie(90, "forward", "residential")}, extra...)...),
		ring(40, 10),
		ring(0, 330, ie(0, "forward", "residential")),
		ring(320, 290),
		ring(280, 250, ie(270, "forward", "residential")),
		ring(240, 210),
		ring(200, 170, ie(180, "forward", "secondary")),
	}
}

func TestFullRoundaboutExits_AllExitsExceptTheRoadWeCameFrom(t *testing.T) {
	// Taking the first exit, only the east exit is known; the lap finds the rest.
	got, ok := FullRoundaboutExits(lap(), RoundaboutExits{Angles: []int{90}})
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{90, 0, 270}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v (the road we came from, at 180, is not an exit)", got.Angles, want)
	}
}

func TestFullRoundaboutExits_LateExit(t *testing.T) {
	got, ok := FullRoundaboutExits(lap(), RoundaboutExits{Angles: []int{90, 0, 270}})
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{90, 0, 270}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestFullRoundaboutExits_LeftHandTraffic(t *testing.T) {
	got, ok := FullRoundaboutExits(mirrored(lap()), RoundaboutExits{Angles: []int{270}})
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{270, 0, 90}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}

func TestFullRoundaboutExits_MustAgreeWithTheRouteLookup(t *testing.T) {
	for name, known := range map[string][]int{
		"another first exit":      {120},
		"more exits than the lap": {90, 0, 270, 200},
		"an exit the lap lacks":   {90, 45},
		"nothing known":           {},
	} {
		if got, ok := FullRoundaboutExits(lap(), RoundaboutExits{Angles: known}); ok {
			t.Errorf("%s: got %+v, want not ok", name, got)
		}
	}
}

func TestFullRoundaboutExits_AnExitInsideTheTolerance(t *testing.T) {
	if _, ok := FullRoundaboutExits(lap(), RoundaboutExits{Angles: []int{97}}); !ok {
		t.Error("7 degrees apart is the same exit")
	}
}

func TestFullRoundaboutExits_NeedsAWholeLap(t *testing.T) {
	// The walk stopped half way round.
	if got, ok := FullRoundaboutExits(lap()[:5], RoundaboutExits{Angles: []int{90}}); ok {
		t.Errorf("got %+v, want not ok", got)
	}
	if _, ok := FullRoundaboutExits([]vhtypes.TraceEdge{road(0, 0), road(0, 0)}, RoundaboutExits{Angles: []int{90}}); ok {
		t.Error("no roundabout: want not ok")
	}
}

func TestFullRoundaboutExits_NotDrivableAndEntryOnlyAreNotExits(t *testing.T) {
	got, ok := FullRoundaboutExits(
		lap(ie(120, "backward", "tertiary"), ie(100, "", "service_other")),
		RoundaboutExits{Angles: []int{90}},
	)
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{90, 0, 270}; !reflect.DeepEqual(got.Angles, want) {
		t.Errorf("got %v, want %v", got.Angles, want)
	}
}
