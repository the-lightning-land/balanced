package bdb

type EdgeId uint64
type ChanId uint64
type Timestamp uint32

type RoutingPolicy struct {
	TimeLockDelta    uint32
	MinHtlc          int64
	FeeBaseMsat      int64
	FeeRateMilliMsat int64
	Disabled         bool
	MaxHtlcMsat      uint64
}

type Edge struct {
	Id         EdgeId
	ChanId     ChanId
	FromNode   PubKey
	ToNode     PubKey
	Capacity   int64
	Policy     *RoutingPolicy
	ChanPoint  string
	LastUpdate uint32
}

type ChannelMap map[ChanId]*Channel

type Channel struct {
	Active       bool
	ChanId       ChanId
	ToNode       PubKey
	LocalBalance uint64
	Capacity     uint64
}
