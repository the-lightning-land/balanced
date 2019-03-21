package balancer

import (
	"github.com/the-lightning-land/balanced/bdb"
	"log"
)

type EdgePath []*bdb.Edge

// findPathsBetween finds a set of edge paths between two given edges
// it currently only supports paths with four edges in total
// and doesn't take the forwarded amount nor fees into consideration
//
// --------    ----------    ---------    ------
// | from | -- | second | -- | third | -- | to |
// --------    ----------    ---------    ------
//
func findPathsBetween(from *bdb.Edge, to *bdb.Edge, graph *bdb.Graph) []EdgePath {
	var paths []EdgePath

	for _, secondEdge := range graph.Edges {
		log.Printf("%v", secondEdge)

		if secondEdge.FromNode == from.ToNode && secondEdge.ToNode == to.FromNode {
			//for _, thirdEdge := range graph.Edges {
			//	if thirdEdge.FromNode == secondEdge.ToNode && thirdEdge.ToNode == to.FromNode {
					paths = append(paths, EdgePath{from, secondEdge, to})
			//	}
			//}
		}
	}

	return paths
}
