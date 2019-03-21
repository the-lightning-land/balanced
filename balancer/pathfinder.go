package balancer

import (
	"github.com/the-lightning-land/balanced/bdb"
)

type EdgePath []*bdb.Edge

// findPathsBetween finds a set of edge paths between two given edges
// it currently only supports paths with four edges in total
// and doesn't take the forwarded amount nor fees into consideration
//
// --------    ----------    / --------- \    ------
// | from | -- | second | -- | | third | | -- | to |
// --------    ----------    \ --------- /    ------
//
func findPathsBetween(from *bdb.Edge, to *bdb.Edge, graph *bdb.Graph) []EdgePath {
	var paths []EdgePath

	for _, secondEdge := range graph.Edges {
		// Does the second node already connect back to destination edge?
		if secondEdge.FromNode == from.ToNode && secondEdge.ToNode == to.FromNode {
			paths = append(paths, EdgePath{from, secondEdge, to})
		}

		//for _, thirdEdge := range graph.Edges {
		//	// Does the third edge connect back to the destination edge?
		//	if thirdEdge.FromNode == secondEdge.ToNode && thirdEdge.ToNode == to.FromNode {
		//		paths = append(paths, EdgePath{from, secondEdge, thirdEdge, to})
		//	}
		//}
	}

	return paths
}
