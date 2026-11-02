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

func TestRoundaboutArms_FourArms(t *testing.T) {
	got, ok := RoundaboutArmsFromEdges(fourArms(), 3)
	if !ok {
		t.Fatal("want ok")
	}
	want := RoundaboutArms{ExitAngles: []int{90, 0, 270}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestRoundaboutArms_EntryOnlyArmIsKept(t *testing.T) {
	// A one-way road into the roundabout that is not the other half of an exit.
	edges := fourArms()
	edges[2].EndNode.IntersectingEdges = append(edges[2].EndNode.IntersectingEdges, ie(30, "backward", "tertiary"))
	got, ok := RoundaboutArmsFromEdges(edges, 3)
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{30}; !reflect.DeepEqual(got.EntryAngles, want) {
		t.Errorf("EntryAngles = %v, want %v", got.EntryAngles, want)
	}
	if want := []int{90, 0, 270}; !reflect.DeepEqual(got.ExitAngles, want) {
		t.Errorf("ExitAngles = %v, want %v", got.ExitAngles, want)
	}
}

func TestRoundaboutArms_OtherHalfOfTwoWayRoadIsMerged(t *testing.T) {
	// The east road is drawn as an exit and, on the same node, an entry.
	got, ok := RoundaboutArmsFromEdges(fourArms(ie(97, "backward", "residential")), 3)
	if !ok {
		t.Fatal("want ok")
	}
	if len(got.EntryAngles) != 0 {
		t.Errorf("EntryAngles = %v, want none", got.EntryAngles)
	}
}

func TestRoundaboutArms_NotDrivableIsIgnored(t *testing.T) {
	// A cycleway has no driveability: not an exit, and Valhalla does not count it.
	got, ok := RoundaboutArmsFromEdges(fourArms(ie(120, "", "service_other")), 3)
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{90, 0, 270}; !reflect.DeepEqual(got.ExitAngles, want) {
		t.Errorf("ExitAngles = %v, want %v", got.ExitAngles, want)
	}
}

func TestRoundaboutArms_DrivewayCounts(t *testing.T) {
	// Valhalla numbers driveways and service roads like any other exit.
	got, ok := RoundaboutArmsFromEdges(fourArms(ie(120, "forward", "service_other")), 4)
	if !ok {
		t.Fatal("want ok")
	}
	if len(got.ExitAngles) != 4 {
		t.Errorf("ExitAngles = %v, want 4 exits", got.ExitAngles)
	}
}

func TestRoundaboutArms_InnerSideIsIgnored(t *testing.T) {
	// 330 leaves on the inner side of the path at the north node.
	edges := fourArms()
	edges[2].EndNode.IntersectingEdges = append(edges[2].EndNode.IntersectingEdges, ie(330, "forward", "residential"))
	if _, ok := RoundaboutArmsFromEdges(edges, 3); !ok {
		t.Error("an inner-side edge must not change the exit count")
	}
}

func TestRoundaboutArms_DisagreeingWithValhallaGivesNothing(t *testing.T) {
	for _, count := range []int{1, 2, 4} {
		if got, ok := RoundaboutArmsFromEdges(fourArms(), count); ok {
			t.Errorf("exit count %d: got %+v, want not ok", count, got)
		}
	}
}

func TestRoundaboutArms_LeftHandTrafficMirrors(t *testing.T) {
	mirror := func(h int) int { return (360 - h) % 360 }
	edges := fourArms()
	for i := range edges {
		edges[i].BeginHeading = mirror(edges[i].BeginHeading)
		edges[i].EndHeading = mirror(edges[i].EndHeading)
		if edges[i].EndNode != nil {
			for j := range edges[i].EndNode.IntersectingEdges {
				edges[i].EndNode.IntersectingEdges[j].BeginHeading = mirror(edges[i].EndNode.IntersectingEdges[j].BeginHeading)
			}
		}
		left := false
		edges[i].DriveOnRight = &left
	}
	got, ok := RoundaboutArmsFromEdges(edges, 3)
	if !ok {
		t.Fatal("want ok")
	}
	if want := []int{270, 0, 90}; !reflect.DeepEqual(got.ExitAngles, want) {
		t.Errorf("ExitAngles = %v, want %v", got.ExitAngles, want)
	}
}

func TestRoundaboutArms_NeedsEntryAndExitEdges(t *testing.T) {
	edges := fourArms()
	if _, ok := RoundaboutArmsFromEdges(edges[1:], 3); ok {
		t.Error("no edge before the roundabout: want not ok")
	}
	if _, ok := RoundaboutArmsFromEdges(edges[:4], 3); ok {
		t.Error("no edge after the roundabout: want not ok")
	}
	if _, ok := RoundaboutArmsFromEdges([]vhtypes.TraceEdge{road(0, 0), road(0, 0)}, 1); ok {
		t.Error("no roundabout: want not ok")
	}
}

// montgomery is a roundabout with split carriageways, reduced to headings,
// driveability and road class: exit 5 of a roundabout entered heading 17.
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

func TestRoundaboutArms_SplitCarriageways(t *testing.T) {
	got, ok := RoundaboutArmsFromEdges(montgomery(), 5)
	if !ok {
		t.Fatal("want ok: the count must match Valhalla's 5th exit")
	}
	if want := []int{91, 11, 357, 276, 260}; !reflect.DeepEqual(got.ExitAngles, want) {
		t.Errorf("ExitAngles = %v, want %v", got.ExitAngles, want)
	}
	if len(got.EntryAngles) != 0 {
		t.Errorf("EntryAngles = %v, want none (all are halves of the two-way roads)", got.EntryAngles)
	}
}
