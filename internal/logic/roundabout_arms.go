package logic

import (
	"sort"

	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
)

// RoundaboutArms are the roads that meet a roundabout, as angles in degrees
// (0-359) clockwise from the direction of travel on entry: 0 is straight on,
// 180 is the road the roundabout was entered from.
type RoundaboutArms struct {
	// ExitAngles are the arms that can be driven out of, in the order the
	// roundabout is passed. The exit taken is among them.
	ExitAngles []int
	// EntryAngles are the arms that can only be driven into the roundabout.
	EntryAngles []int
}

// pairHeadingDegrees is how far apart the two halves of one two-way road may
// point, where it is drawn as two one-way roads on neighbouring nodes.
const pairHeadingDegrees = 60

// RoundaboutArmsFromEdges works out the arms of the roundabout that the edges
// of a matched shape pass through. The edges must run from the road before
// the roundabout, through the roundabout, to the road after it.
//
// The exits are found with the same rule Valhalla uses to number them: on
// every roundabout node, each intersecting edge on the outer side of the
// path (the right where traffic drives on the right) that can be driven out
// of the roundabout is an exit, plus the exit taken. [exitCount] is Valhalla's
// own count for the maneuver; when the result disagrees with it, or the exit
// taken is not at that position, the second result is false and nothing
// should be used.
func RoundaboutArmsFromEdges(
	edges []vhtypes.TraceEdge,
	exitCount int,
) (RoundaboutArms, bool) {
	first, last := -1, -1
	for i, e := range edges {
		if !e.Roundabout {
			if first >= 0 {
				break
			}
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 1 || last+1 >= len(edges) {
		return RoundaboutArms{}, false
	}

	driveOnRight := edges[first].DriveOnRight == nil || *edges[first].DriveOnRight
	entryHeading := edges[first-1].EndHeading

	var exits, entries []roundaboutArm
	for i := first; i <= last; i++ {
		e := edges[i]
		if e.EndNode == nil {
			continue
		}
		path := turnDegrees(e.EndHeading, edges[i+1].BeginHeading)
		for _, ie := range e.EndNode.IntersectingEdges {
			t := turnDegrees(e.EndHeading, ie.BeginHeading)
			if !outerSide(t, path, driveOnRight) {
				continue
			}
			a := roundaboutArm{angle: turnDegrees(entryHeading, ie.BeginHeading), node: i, class: ie.RoadClass}
			switch ie.Driveability {
			case vhtypes.DriveForward, vhtypes.DriveBoth:
				exits = append(exits, a)
			case vhtypes.DriveBackward:
				entries = append(entries, a)
			}
		}
	}

	// Valhalla's count: the exit taken plus every exit passed on the way.
	if len(exits)+1 != exitCount {
		return RoundaboutArms{}, false
	}
	taken := roundaboutArm{
		angle: turnDegrees(entryHeading, edges[last+1].BeginHeading),
		node:  last,
		class: edges[last+1].RoadClass,
	}
	exits = append(exits, taken)

	key := func(angle int) int { return armOrderKey(angle, driveOnRight) }
	sort.SliceStable(exits, func(i, j int) bool { return key(exits[i].angle) < key(exits[j].angle) })

	// The exit taken has to be the exitCount-th one passed.
	before, same := 0, 0
	for _, x := range exits {
		switch k := key(x.angle); {
		case k < key(taken.angle):
			before++
		case k == key(taken.angle):
			same++
		}
	}
	if exitCount < before+1 || exitCount > before+same {
		return RoundaboutArms{}, false
	}

	var result RoundaboutArms
	for _, x := range exits {
		result.ExitAngles = append(result.ExitAngles, x.angle)
	}
	sort.SliceStable(entries, func(i, j int) bool { return key(entries[i].angle) < key(entries[j].angle) })
	for _, en := range entries {
		if isOtherHalfOfExit(en, exits) {
			continue
		}
		result.EntryAngles = append(result.EntryAngles, en.angle)
	}
	return result, true
}

// roundaboutArm is a road meeting a roundabout: its angle, the roundabout
// node it leaves from (the index of the edge that ends there) and its class.
type roundaboutArm struct {
	angle int
	node  int
	class string
}

// isOtherHalfOfExit reports whether an entry-only arm is the second half of a
// two-way road whose first half is an exit: Valhalla draws such a road as two
// one-way roads on neighbouring nodes.
func isOtherHalfOfExit(entry roundaboutArm, exits []roundaboutArm) bool {
	for _, x := range exits {
		if x.node-entry.node > 1 || entry.node-x.node > 1 || x.class != entry.class {
			continue
		}
		if headingDistance(x.angle, entry.angle) <= pairHeadingDegrees {
			return true
		}
	}
	return false
}

// armOrderKey orders angles in the order a roundabout is passed, starting at
// the road it was entered from: counter-clockwise where traffic drives on the
// right, clockwise where it drives on the left.
func armOrderKey(angle int, driveOnRight bool) int {
	if driveOnRight {
		return ((180-angle)%360 + 360) % 360
	}
	return ((angle-180)%360 + 360) % 360
}

// outerSide mirrors Valhalla's test for an intersecting edge on the outer side
// of the path: right of it where traffic drives on the right, left otherwise.
// [t] and [path] are turn degrees from the heading on arrival.
func outerSide(t, path int, driveOnRight bool) bool {
	if driveOnRight {
		if path > 180 {
			return t > path || t < 180
		}
		return t > path && t < 180
	}
	if path > 180 {
		return t < path && t > 180
	}
	return t < path || t > 180
}

// turnDegrees is the clockwise turn from heading [from] to heading [to].
func turnDegrees(from, to int) int {
	return ((to-from)%360 + 360) % 360
}

// headingDistance is the smallest angle between two headings.
func headingDistance(a, b int) int {
	d := turnDegrees(a, b)
	if d > 180 {
		d = 360 - d
	}
	return d
}
