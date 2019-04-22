package balancer

import (
	"github.com/the-lightning-land/balanced/bdb"
)

type EdgePath []*bdb.Edge

// findPathsBetween finds a set of edge paths between two given edges
// it currently only supports paths with four edges in total
// and doesn't take the forwarded amount nor fees into consideration
//
//              /                             \
// --------    /  ----------    / --------- \  \    ------
// | from | -- |  | second | -- | | third | |  | -- | to |
// --------    \  ----------    \ --------- /  /    ------
//              \                             /
//
func findPathsBetween(from *bdb.Edge, to *bdb.Edge, graph *bdb.Graph) []EdgePath {
	var paths []EdgePath
	var firstPriority []EdgePath
	var secondPriority []EdgePath
	var thirdPriority []EdgePath

	if from.ToNode == to.FromNode {
		firstPriority = append(firstPriority, EdgePath{from, to})
	}

	for _, secondEdge := range graph.Edges {
		// Does the second node already connect back to destination edge?
		if secondEdge.FromNode == from.ToNode && secondEdge.ToNode == to.FromNode {
			secondPriority = append(secondPriority, EdgePath{from, secondEdge, to})
		}

		if secondEdge.FromNode == from.ToNode {
			for _, thirdEdge := range graph.Edges {
				// Does the third edge connect back to the destination edge?
				if thirdEdge.FromNode == secondEdge.ToNode && thirdEdge.ToNode == to.FromNode {
					thirdPriority = append(thirdPriority, EdgePath{from, secondEdge, thirdEdge, to})
				}
			}
		}
	}

	paths = append(paths, firstPriority...)
	paths = append(paths, secondPriority...)
	paths = append(paths, thirdPriority...)

	return paths
}
