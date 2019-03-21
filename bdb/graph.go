package bdb

type EdgeMap map[EdgeId]*Edge
type NodeMap map[PubKey]*Node

type Graph struct {
	Edges EdgeMap
	Nodes NodeMap
}
