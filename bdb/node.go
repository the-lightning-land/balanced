package bdb

type PubKey string

type Node struct {
	PubKey PubKey
	Alias  string
}
