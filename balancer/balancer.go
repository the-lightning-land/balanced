package balancer

import (
	"fmt"
	"github.com/go-errors/errors"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/the-lightning-land/balanced/bdb"
	"github.com/the-lightning-land/balanced/lndc"
	"time"
)

type Balancer struct {
	logger         Logger
	done           chan struct{}
	client         *lndc.Client
	cachedInvoices []*bdb.Invoice
	identityPubKey bdb.PubKey
	graph          *bdb.Graph
	channelMap     bdb.ChannelMap
}

type Config struct {
	Logger Logger
	Client *lndc.Client
}

func NewBalancer(config *Config) (error, *Balancer) {
	balancer := &Balancer{}

	if config.Logger != nil {
		balancer.logger = config.Logger
	} else {
		balancer.logger = noopLogger{}
	}

	balancer.done = make(chan struct{})
	balancer.client = config.Client

	return nil, balancer
}

func (b *Balancer) IdentityPubKey() (bdb.PubKey, error) {
	identityPubKey, err := b.client.IdentitiyPubKey()
	if err != nil {
		return "", errors.Errorf("Could not get identity pubkey: %v", err)
	}

	return identityPubKey, nil
}

func (b *Balancer) Run() error {
	var err error

	b.identityPubKey, err = b.client.IdentitiyPubKey()
	if err != nil {
		return errors.Errorf("Could not identity pub key: %v", err)
	}

	b.graph, err = b.client.Graph()
	if err != nil {
		return errors.Errorf("Could not get graph: %v", err)
	}

	b.channelMap, err = b.client.Channels()
	if err != nil {
		return errors.Errorf("Could not get channels: %v", err)
	}

	b.logger.Infof("Ready.")

	// b.BalanceAll()

	for {
		select {
		case <-b.done:
			b.logger.Infof("Stopping balancer...")
			return nil
		}
	}
}

func (b *Balancer) BalanceFull(amtMsat int64, fullChanId bdb.ChanId) error {
	var fullChannel *bdb.Channel
	var emptyChannels []*bdb.Channel

	for _, channel := range b.channelMap {
		if channel.ChanId == fullChanId {
			fullChannel = channel
		} else if channel.LocalBalance < channel.Capacity*20/100 {
			emptyChannels = append(emptyChannels, channel)
		}
	}

	b.logger.Infof("Found %v empty channels", len(emptyChannels))

	var fullChannelEdge *bdb.Edge

	for _, edge := range b.graph.Edges {
		if edge.ChanId == fullChanId && edge.FromNode == b.identityPubKey {
			fullChannelEdge = edge
		}
	}

	if fullChannelEdge == nil {
		return errors.Errorf("Could not find full channel edge %v in graph", fullChanId)
	}

	b.logger.Infof("Starting to balance full (%v/%v) channel %v to node %v",
		fullChannel.LocalBalance, fullChannel.Capacity, fullChannel.ChanId, b.graph.Nodes[fullChannel.ToNode].Alias)

	for _, emptyChannel := range emptyChannels {
		var emptyChannelEdge *bdb.Edge

		for _, edge := range b.graph.Edges {
			if edge.ChanId == emptyChannel.ChanId && edge.ToNode == b.identityPubKey {
				emptyChannelEdge = edge
			}
		}

		if emptyChannelEdge == nil {
			b.logger.Infof("Could not find empty channel edge %v in graph", emptyChannel.ChanId)
			continue
		}

		rebalanced, err := b.balance(amtMsat, fullChannel, fullChannelEdge, emptyChannel, emptyChannelEdge, b.graph)
		if err != nil {
			return errors.Errorf("Could not rebalance: %v", err)
		}

		if rebalanced {
			break
		}
	}

	return nil
}

func (b *Balancer) BalanceAll(amtMsat int64) error {
	var fullChannels []*bdb.Channel
	var emptyChannels []*bdb.Channel

	for _, channel := range b.channelMap {
		if channel.LocalBalance > channel.Capacity*80/100 {
			fullChannels = append(fullChannels, channel)
		} else if channel.LocalBalance < channel.Capacity*20/100 {
			emptyChannels = append(emptyChannels, channel)
		}
	}

	b.logger.Infof("Found %v full and %v empty channels", len(fullChannels), len(emptyChannels))

	for _, fullChannel := range fullChannels {
		var fullChannelEdge *bdb.Edge

		for _, edge := range b.graph.Edges {
			if edge.ChanId == fullChannel.ChanId && edge.FromNode == b.identityPubKey {
				fullChannelEdge = edge
			}
		}

		if fullChannelEdge == nil {
			b.logger.Infof("Could not find full channel edge %v in graph", fullChannel.ChanId)
			continue
		}

		b.logger.Infof("Starting to balance full (%v/%v) channel %v to node %v",
			fullChannel.LocalBalance, fullChannel.Capacity, fullChannel.ChanId, b.graph.Nodes[fullChannel.ToNode].Alias)

		for _, emptyChannel := range emptyChannels {
			var emptyChannelEdge *bdb.Edge

			for _, edge := range b.graph.Edges {
				if edge.ChanId == emptyChannel.ChanId && edge.ToNode == b.identityPubKey {
					emptyChannelEdge = edge
				}
			}

			if emptyChannelEdge == nil {
				b.logger.Infof("Could not find empty channel edge %v in graph", emptyChannel.ChanId)
				continue
			}

			rebalanced, err := b.balance(amtMsat, fullChannel, fullChannelEdge, emptyChannel, emptyChannelEdge, b.graph)
			if err != nil {
				return errors.Errorf("Could not rebalance: %v", err)
			}

			if rebalanced {
				break
			}
		}
	}

	return nil
}

func (b *Balancer) Balance(amtMsat int64, fullChanId bdb.ChanId, emptyChanId bdb.ChanId) (bool, error) {
	var fullChannel *bdb.Channel
	var emptyChannel *bdb.Channel

	for _, channel := range b.channelMap {
		if channel.ChanId == fullChanId {
			fullChannel = channel
		} else if channel.ChanId == emptyChanId {
			emptyChannel = channel
		}
	}

	if fullChannel == nil {
		return false, errors.Errorf("Could not find full channel: %v", fullChanId)
	}

	if emptyChannel == nil {
		return false, errors.Errorf("Could not find empty channel: %v", emptyChanId)
	}

	var fullChannelEdge *bdb.Edge
	var emptyChannelEdge *bdb.Edge

	for _, edge := range b.graph.Edges {
		if edge.ChanId == fullChannel.ChanId && edge.FromNode == b.identityPubKey {
			fullChannelEdge = edge
		} else if edge.ChanId == emptyChannel.ChanId && edge.ToNode == b.identityPubKey {
			emptyChannelEdge = edge
		}
	}

	b.logger.Infof("Balancing between %v and %v", fullChannelEdge.ChanId, emptyChannelEdge.ChanId)

	rebalanced, err := b.balance(amtMsat, fullChannel, fullChannelEdge, emptyChannel, emptyChannelEdge, b.graph)
	if err != nil {
		return false, errors.Errorf("Could not rebalance: %v", err)
	}

	return rebalanced, nil
}

func (b *Balancer) balance(amtMsat int64, fullChannel *bdb.Channel, fullChannelEdge *bdb.Edge,
	emptyChannel *bdb.Channel, emptyChannelEdge *bdb.Edge, graph *bdb.Graph) (bool, error) {

	b.logger.Infof("Searching paths to empty (%v/%v) channel %v with %v",
		emptyChannel.LocalBalance, emptyChannel.Capacity, emptyChannel.ChanId, graph.Nodes[emptyChannel.ToNode].Alias)

	b.logger.Infof("We've got %v graph edges", len(graph.Edges))

	b.logger.Infof("From node %v to node %v", fullChannelEdge.ToNode, emptyChannelEdge.FromNode)

	edgePaths := findPathsBetween(fullChannelEdge, emptyChannelEdge, graph)

	b.logger.Infof("Found %v paths", len(edgePaths))

	for i, edgePath := range edgePaths {
		b.logger.Infof("Trying path %v/%v...", i+1, len(edgePaths))

		lndRoute, err := b.ConstructRoute(edgePath, amtMsat)
		if err != nil {
			b.logger.Infof("Could not construct route: %v", err)
			continue
		}

		for amtMsat = amtMsat; amtMsat > 50000; amtMsat = amtMsat / 2 {
			b.logger.Infof("Attempting rebalance with %v sats", amtMsat/1000)

			invoice, err := b.AddOrReuseInvoice(amtMsat)
			if err != nil {
				b.logger.Infof("Could not add new invoice: %v", err)
				continue
			}

			payment, err := b.PayInvoiceThroughRoute(invoice, lndRoute)
			if err != nil {
				b.logger.Infof("Could not pay invoice: %v", err)
				break
			}

			b.logger.Infof("Successfully balanced with payment %v", payment)

			// Adjust the balances approximately, not considering fees for now
			fullChannel.LocalBalance -= uint64(amtMsat) / 1000
			emptyChannel.LocalBalance += uint64(amtMsat) / 1000

			// Decide whether it's still necessary to continue rebalancing of the current full channel
			if fullChannel.LocalBalance < fullChannel.Capacity*80/100 {
				b.logger.Infof("Rebalanced full (%v/%v) channel %v to node %v",
					fullChannel.LocalBalance, fullChannel.Capacity, fullChannel.ChanId, graph.Nodes[fullChannel.ToNode].Alias)
				return true, nil
			}
		}
	}

	return false, nil
}

func (b *Balancer) Stop() {
	close(b.done)
}

func (b *Balancer) AddOrReuseInvoice(amtMsat int64) (*bdb.Invoice, error) {
	// Try to return an already created invoice that isn't settled yet
	for _, invoice := range b.cachedInvoices {
		if invoice.NumSatoshis == amtMsat/1000 && invoice.Expiry < time.Now().Unix() {
			b.logger.Infof("Reusing invoice of %v satoshis", amtMsat/1000)
			return invoice, nil
		}
	}

	invoice, err := b.client.AddInvoice(amtMsat)
	if err != nil {
		return nil, errors.Errorf("Could not add invoice: %v", err)
	}

	b.logger.Infof("Adding invoice of %v satoshis", amtMsat/1000)

	b.cachedInvoices = append(b.cachedInvoices, invoice)

	return invoice, nil
}

func (b *Balancer) PayInvoiceThroughRoute(invoice *bdb.Invoice, lndRoute *lnrpc.Route) (*bdb.Payment, error) {
	payment, err := b.client.SendToRoute(&lnrpc.SendToRouteRequest{
		PaymentHashString: invoice.PaymentHash,
		Routes:            []*lnrpc.Route{lndRoute},
	})
	if err != nil {
		return nil, errors.Errorf("Could not send payment: %v", err)
	}

	invoiceIndex := int(-1)
	for i, cachedInvoice := range b.cachedInvoices {
		if cachedInvoice.PaymentHash == invoice.PaymentHash {
			invoiceIndex = i
			break
		}
	}

	if invoiceIndex >= 0 {
		// Remove the settled invoice from invoices cache
		b.cachedInvoices = append(b.cachedInvoices[:invoiceIndex], b.cachedInvoices[invoiceIndex+1:]...)
	}

	return payment, nil
}

// ConstructRoute builds a fully configured route (currently for lnd only)
// given a path of directed edges and the amount that should be transferred
func (b *Balancer) ConstructRoute(edgePath EdgePath, amt int64) (*lnrpc.Route, error) {
	identityPubKey, err := b.client.IdentitiyPubKey()
	if err != nil {
		return nil, errors.Errorf("Could not get identity pubkey: %v", err)
	}

	blockHeight, err := b.client.BlockHeight()
	if err != nil {
		return nil, errors.Errorf("Could not get block height: %v", err)
	}

	forwardingToPubKey := identityPubKey
	hops := make([]*lnrpc.Hop, len(edgePath))
	var totalFeesMsat int64 = 0
	var totalAmtMsat = amt
	var totalTimeLock = uint32(blockHeight)

	// Go through the path in reverse, making it easier to calculate the forwarded amount
	for i := len(edgePath) - 1; i >= 0; i-- {
		edge := edgePath[i]

		var policy *bdb.RoutingPolicy
		if i+1 < len(edgePath) {
			edge := edgePath[i+1]
			policy = edge.Policy
		}

		feeMsat := int64(0)
		if policy != nil {
			edge := edgePath[i+1]
			policy := edge.Policy

			feeMsat = policy.FeeBaseMsat + (totalAmtMsat*policy.FeeRateMilliMsat)/1000000
		}

		var expiry uint32

		if policy != nil {
			totalTimeLock += policy.TimeLockDelta
			expiry = totalTimeLock - policy.TimeLockDelta
		} else {
			totalTimeLock += 14
			expiry = uint32(blockHeight) + 14
		}

		hops[i] = &lnrpc.Hop{
			PubKey:           string(forwardingToPubKey),
			FeeMsat:          feeMsat,
			Expiry:           expiry,
			ChanCapacity:     edge.Capacity,
			AmtToForwardMsat: totalAmtMsat,
			ChanId:           uint64(edge.ChanId),
		}

		totalFeesMsat += feeMsat
		totalAmtMsat += feeMsat

		if totalAmtMsat/1000 > edge.Capacity {
			return nil, errors.Errorf("Won't be able to send %v when capacity of edge %v is only %v", totalAmtMsat/1000, edge.ChanId, edge.Capacity)
		}

		forwardingToPubKey = edge.FromNode
	}

	if totalFeesMsat > amt/10 {
		return nil, errors.Errorf("Fee %v larger than 10%% of the amount %v", totalFeesMsat, amt)
	}

	route := &lnrpc.Route{
		TotalAmtMsat:  totalAmtMsat,
		TotalFeesMsat: totalFeesMsat,
		TotalTimeLock: totalTimeLock,
		Hops:          hops,
	}

	fmt.Println("Total fees and amount:", totalFeesMsat, totalAmtMsat)

	return route, nil
}
