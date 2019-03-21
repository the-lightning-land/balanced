package balancer

import (
	"github.com/the-lightning-land/balanced/bdb"
	"testing"
)

func TestPathfinder(t *testing.T) () {
	edgeMap := make(bdb.EdgeMap)

	edgeMap[0] = &bdb.Edge{
		FromNode: "Alice",
		ToNode: "Bob",
	}

	edgeMap[1] = &bdb.Edge{
		FromNode: "Bob",
		ToNode: "Charlie",
	}

	edgeMap[2] = &bdb.Edge{
		FromNode: "Charlie",
		ToNode: "Alice",
	}

	edgeMap[3] = &bdb.Edge{
		FromNode: "Charlie",
		ToNode: "Dan",
	}

	edgeMap[4] = &bdb.Edge{
		FromNode: "Bob",
		ToNode: "Dan",
	}

	graph := &bdb.Graph{
		Edges: edgeMap,
	}

	edgePaths := findPathsBetween(edgeMap[0], edgeMap[2], graph)

	if len(edgePaths) != 1 {
		t.Errorf("One path expected; got %v", len(edgePaths))
	}
}