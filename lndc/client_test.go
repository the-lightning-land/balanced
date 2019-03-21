package lndc

import (
	"context"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/kr/pretty"
	"github.com/lightningnetwork/lnd/lnrpc"
	log "github.com/sirupsen/logrus"
	"github.com/the-lightning-land/balanced/bdb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"hash/fnv"
	"io"
	"os"
	"testing"
)

func TestParseError(t *testing.T) {
	paymentError := "unable to route payment to destination: TemporaryChannelFailure(update=(*lnwire.ChannelUpdate)(0xcbe3cc0)({\n Signature: (lnwire.Sig) (len=64 cap=64) {\n  00000000  0b d4 c9 7f 09 ab 1d 3c  5b db a3 14 15 f9 5d 1f  |.......<[.....].|\n  00000010  27 79 2c 0b c5 b9 fb 01  da f1 17 4e f9 af 89 b5  |'y,........N....|\n  00000020  5a 17 3c f6 ea d1 af 9f  1b 85 12 2a 7b 13 9a 89  |Z.<........*{...|\n  00000030  f4 cf a8 83 6d f8 04 fd  e4 44 0b 57 34 d2 e7 10  |....m....D.W4...|\n },\n ChainHash: (chainhash.Hash) (len=32 cap=32) 000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f,\n ShortChannelID: (lnwire.ShortChannelID) 557807:665:1,\n Timestamp: (uint32) 1553189282,\n MessageFlags: (lnwire.ChanUpdateMsgFlags) 00000001,\n ChannelFlags: (lnwire.ChanUpdateChanFlags) 00000001,\n TimeLockDelta: (uint16) 144,\n HtlcMinimumMsat: (lnwire.MilliSatoshi) 1000 mSAT,\n BaseFee: (uint32) 0,\n FeeRate: (uint32) 1,\n HtlcMaximumMsat: (lnwire.MilliSatoshi) 1000000000 mSAT,\n ExtraOpaqueData: ([]uint8) <nil>\n})\n)"
	err := mapPaymentErrorToTypedError(paymentError)

	switch err := err.(type) {
	case bdb.TemporaryChannelFailureError:
		if err.ChanId != 613315282598428673 {
			t.Errorf("Expected channel id of 613315282598428673; got %v", err.ChanId)
		}
	default:
		t.Errorf("Expected temporary channel failure; got %T", err)
	}
}

var client lnrpc.LightningClient
var ctx context.Context

func Connect() (lnrpc.LightningClient, context.Context, error) {
	if client != nil && ctx != nil {
		// return previously created connection
		return client, ctx, nil
	}

	cert, err := makeTlsCertFromPath("/Users/davidknezic/Documents/lnd/the.lightning.land/tls.cert")
	if err != nil {
		return nil, nil, errors.Errorf("Could not make TLS cert: %v", err)
	}

	creds := credentials.NewClientTLSFromCert(cert, "")

	conn, err := grpc.Dial("192.168.2.101:10009", grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, nil, errors.Errorf("Could not connect to lightning node: %v", err)
	}

	client = lnrpc.NewLightningClient(conn)

	macaroon, err := makeMacaroonFromPath("/Users/davidknezic/Documents/lnd/the.lightning.land/admin.macaroon")
	if err != nil {
		return nil, nil, errors.Errorf("Could not make macaroon: %v", err)
	}

	ctx = context.Background()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("macaroon", macaroon))

	return client, ctx, nil
}

func TestGetInfo(t *testing.T) () {
	client, ctx, err := Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	info, err := client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		log.Fatalf("Could not get info: %v", err)
	}

	log.Infof("My identity pubkey: %v", info.IdentityPubkey)
}

type DirectedEdge struct {
	ChannelId                 uint64
	TimeLockDelta             uint32
	FeeBaseMSat               int64
	FeeProportionalMillionths int64
	PubKey                    string
	Capacity                  int64
}

func directChannelPathEdges(sourcePubKey string, channelPath []uint64, edgeMap map[uint64]*lnrpc.ChannelEdge) []*DirectedEdge {
	directedEdgePath := make([]*DirectedEdge, len(channelPath))

	for i, channelId := range channelPath {
		channel := edgeMap[channelId]

		var policy *lnrpc.RoutingPolicy
		var pubKey string

		pretty.Print(channel)

		if channel.Node1Pub == sourcePubKey {
			log.Infof("%v == %v = true", channel.Node1Pub, sourcePubKey)

			policy = channel.Node2Policy
			pubKey = channel.Node2Pub
		} else {
			log.Infof("%v == %v = false", channel.Node1Pub, sourcePubKey)

			policy = channel.Node1Policy
			pubKey = channel.Node1Pub
		}

		log.Infof("Using policy with node %v", pubKey)

		directedEdgePath[i] = &DirectedEdge{
			ChannelId:                 channelId,
			TimeLockDelta:             policy.TimeLockDelta,
			FeeBaseMSat:               policy.FeeBaseMsat,
			FeeProportionalMillionths: policy.FeeRateMilliMsat,
			PubKey:                    pubKey,
			Capacity:                  channel.Capacity,
		}

		sourcePubKey = pubKey
	}

	return directedEdgePath
}

func computeFee(amt int64, edge *DirectedEdge) int64 {
	fee := edge.FeeBaseMSat + (amt*edge.FeeProportionalMillionths)/1000000
	log.Infof("Fee %v := %v + (%v * %v) / 1000000", fee, edge.FeeBaseMSat, amt, edge.FeeProportionalMillionths)
	// fee += 1000
	return fee
}

func TestDirectConnection(t *testing.T) () {
	from := "0374ecf61ed6c1208c42339f47decde2bc0c4393ac95f07827b3471e939d7eb961"
	to := ""

	client, ctx, err := Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	channelGraph, err := client.DescribeGraph(ctx, &lnrpc.ChannelGraphRequest{
		IncludeUnannounced: true,
	}, grpc.MaxCallRecvMsgSize(50*1024*1024))
	if err != nil {
		log.Fatalf("Could not get channel graph: %v", err)
	}

	// Create a node map for easier lookup of aliases
	nodeMap := make(map[string]*lnrpc.LightningNode, len(channelGraph.Nodes))
	for _, node := range channelGraph.Nodes {
		nodeMap[node.PubKey] = node
	}

	channelList, err := client.ListChannels(ctx, &lnrpc.ListChannelsRequest{
		ActiveOnly: true,
	})
	if err != nil {
		log.Fatalf("Could not list channels: %v", err)
	}

	for _, edge := range channelGraph.Edges {
		if from != "" && to != "" {
			// when both from and to are provided

			if edge.Node1Pub == from && edge.Node2Pub == to || edge.Node1Pub == to && edge.Node2Pub == from {
				log.Infof("Found a direct link %v with capacity of %v mSat", edge.ChannelId, edge.Capacity)
			}
		} else if from != "" && to == "" {
			// when only from is provided

			if edge.Node1Pub == from {
				for _, channel := range channelList.Channels {
					if channel.RemotePubkey == edge.Node2Pub {
						log.Infof("Found a link %v with capacity of %v mSat via %v (%v) with remote balance %v", edge.ChannelId, edge.Capacity, channel.ChanId, nodeMap[channel.RemotePubkey].Alias, channel.RemoteBalance)
					}
				}
			} else if edge.Node2Pub == from {
				for _, channel := range channelList.Channels {
					if channel.RemotePubkey == edge.Node1Pub {
						log.Infof("Found a link %v with capacity of %v mSat via %v (%v) with remote balance %v", edge.ChannelId, edge.Capacity, channel.ChanId, nodeMap[channel.RemotePubkey].Alias, channel.RemoteBalance)
					}
				}
			}
		} else if from == "" && to != "" {
			// when only to is provided
		}
	}
}

func TestSendToRoute(t *testing.T) () {
	var amtToSend int64 = 10000 * 1000
	channelPath := []uint64{621791417835454464, 618904100220567552, 616646802897829888}

	client, ctx, err := Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	info, err := client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		log.Fatalf("Could not get info: %v", err)
	}

	log.Infof("Current block is: %v", info.BlockHeight)

	addInvoice, err := client.AddInvoice(ctx, &lnrpc.Invoice{
		CltvExpiry: 9,
		Value:      int64(0.001 * float32(amtToSend)),
	})
	if err != nil {
		log.Fatalf("Could not add invoice: %v", err)
	}

	log.Printf("Invoice: %v", addInvoice.PaymentRequest)

	decodedInvoice, err := client.DecodePayReq(ctx, &lnrpc.PayReqString{
		PayReq: addInvoice.PaymentRequest,
	})
	if err != nil {
		log.Fatalf("Could not decode invoice: %v", err)
	}

	pretty.Print(decodedInvoice)

	log.Infof("Successfully created invoice: %v", hex.EncodeToString(addInvoice.RHash))

	channelGraph, err := client.DescribeGraph(ctx, &lnrpc.ChannelGraphRequest{
		IncludeUnannounced: true,
	}, grpc.MaxCallRecvMsgSize(50*1024*1024))
	if err != nil {
		log.Fatalf("Could not get channel graph: %v", err)
	}

	edgeMap := make(map[uint64]*lnrpc.ChannelEdge, len(channelGraph.Edges))
	for _, edge := range channelGraph.Edges {
		edgeMap[edge.ChannelId] = edge
	}

	forwardingToPubKey := info.IdentityPubkey
	hops := make([]*lnrpc.Hop, len(channelPath))
	var total_fees_msat int64 = 0
	var total_amt_msat = amtToSend
	var totalTimeLock = info.BlockHeight

	// Go through the path in reverse, making it easier to calculate the forwarded amount
	for i := len(channelPath) - 1; i >= 0; i-- {
		channel := channelPath[i]
		edge := edgeMap[channel]

		if edge == nil {
			log.Fatalf("Could not get edge %v", channel)
		}

		// TODO: Is the right channel policy chosen here?
		var policy *lnrpc.RoutingPolicy
		if edge.Node1Pub != forwardingToPubKey {
			policy = edge.Node2Policy
			log.Infof("Chosing fee policy base: %v, rate: %v", edge.Node2Policy.FeeBaseMsat, edge.Node2Policy.FeeRateMilliMsat)
			log.Infof("And not fee policy base: %v, rate: %v", edge.Node1Policy.FeeBaseMsat, edge.Node1Policy.FeeRateMilliMsat)
		} else {
			policy = edge.Node1Policy
			log.Infof("Chosing fee policy base: %v, rate: %v", edge.Node1Policy.FeeBaseMsat, edge.Node1Policy.FeeRateMilliMsat)
			log.Infof("And not fee policy base: %v, rate: %v", edge.Node2Policy.FeeBaseMsat, edge.Node2Policy.FeeRateMilliMsat)
		}

		log.Infof("Time lock delta is %v", policy.TimeLockDelta)
		log.Infof("Got policy %v %v", policy.FeeBaseMsat, policy.FeeRateMilliMsat)

		if policy.Disabled {
			log.Fatalf("Oh no, edge %v towards %v is disabled!", edge.ChannelId, forwardingToPubKey)
		}

		feeMsat := policy.FeeBaseMsat + (total_amt_msat*policy.FeeRateMilliMsat)/1000000
		log.Infof("Fee %v := %v + (%v * %v) / 1000000", feeMsat, policy.FeeBaseMsat, total_amt_msat, policy.FeeRateMilliMsat)

		var expiry uint32

		if forwardingToPubKey == info.IdentityPubkey {
			totalTimeLock += 9
			expiry = info.BlockHeight + 9
		} else {
			totalTimeLock += policy.TimeLockDelta
			expiry = totalTimeLock - policy.TimeLockDelta
		}

		log.Infof("#%v: CLTV expiry: %v", i+1, expiry)

		hops[i] = &lnrpc.Hop{
			PubKey:           forwardingToPubKey,
			FeeMsat:          feeMsat,
			Expiry:           expiry,
			ChanCapacity:     edge.Capacity,
			AmtToForwardMsat: total_amt_msat,
			ChanId:           channel,
		}

		total_fees_msat += feeMsat
		total_amt_msat += feeMsat

		if total_amt_msat/1000 > edge.Capacity {
			log.Fatalf("Won't be able to send %v when capacity of edge %v is only %v", total_amt_msat/1000, edge.ChannelId, edge.Capacity)
		}

		// set the previous node pub key which needs forwarding to
		if edge.Node1Pub == forwardingToPubKey {
			forwardingToPubKey = edge.Node2Pub
		} else {
			forwardingToPubKey = edge.Node1Pub
		}
	}

	log.Infof("Sending payment off through hops %v", hops)

	log.Infof("Fee is %v msat", total_fees_msat)

	if total_fees_msat > amtToSend/10 {
		log.Fatalf("Fee %v larger than 10%% of the amount %v", total_fees_msat, amtToSend)
	}

	route := &lnrpc.Route{
		TotalAmtMsat:  total_amt_msat,
		TotalFeesMsat: total_fees_msat,
		TotalTimeLock: totalTimeLock,
		Hops:          hops,
	}

	sendResult, err := client.SendToRouteSync(ctx, &lnrpc.SendToRouteRequest{
		PaymentHashString: hex.EncodeToString(addInvoice.RHash),
		Routes:            []*lnrpc.Route{route},
	})
	if err != nil {
		log.Fatalf("Could not send to route: %v", err)
	}

	if sendResult.PaymentError != "" {
		t.Fatalf("Could not send payment to route: %v", sendResult.PaymentError)
	}

	t.Logf("Successfully sent to route with preimage %v", sendResult.PaymentPreimage)
}

func TestAddInvoice(t *testing.T) () {
	client, ctx, err := Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	addInvoice, err := client.AddInvoice(ctx, &lnrpc.Invoice{
		AmtPaidSat: 100000,
	})
	if err != nil {
		log.Fatalf("Could not add invoice: %v", err)
	}

	log.Infof("Successfully created invoice: %v", hex.EncodeToString(addInvoice.RHash))
}

func TestHashCollisions(t *testing.T) () {
	client, ctx, err := Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	channelGraph, err := client.DescribeGraph(ctx, &lnrpc.ChannelGraphRequest{}, grpc.MaxCallRecvMsgSize(50*1024*1024))
	if err != nil {
		log.Fatalf("Could not get channel graph: %v", err)
	}

	h := fnv.New64a()
	set := make(map[int64]string)

	for _, node := range channelGraph.Nodes {
		h.Write([]byte(node.PubKey))
		i := int64(h.Sum64())
		h.Reset()

		pubKey, found := set[i]
		if found {
			log.Fatalf("Found a collision: hash(%v) = hash(%v)", node.PubKey, pubKey)
		}

		set[i] = node.PubKey
	}

	log.Infof("Successfully checked for no collisions")
}

type GexfRoot struct {
	XMLName xml.Name `xml:"gexf"`
	Xmlns   string   `xml:"xmlns,attr"`
	Version string   `xml:"version,attr"`
	Graph   *GraphRoot
}

type GraphRoot struct {
	XMLName         xml.Name    `xml:"graph"`
	Mode            string      `xml:"mode,attr"`
	DefaultEdgeType string      `xml:"defaultedgetype,attr"`
	Nodes           *GraphNodes `xml:"nodes"`
	Edges           *GraphEdges `xml:"edges"`
}

type GraphNodes struct {
	XMLName xml.Name     `xml:"nodes"`
	Items   []*GraphNode `xml:"items"`
}

type GraphNode struct {
	XMLName xml.Name `xml:"node"`
	ID      string   `xml:"id,attr"`
	Label   string   `xml:"label,attr"`
}

type GraphEdges struct {
	XMLName xml.Name     `xml:"edges"`
	Items   []*GraphEdge `xml:"items"`
}

type GraphEdge struct {
	XMLName          xml.Name `xml:"edge"`
	ID               string   `xml:"id,attr"`
	Source           string   `xml:"source,attr"`
	Target           string   `xml:"target,attr"`
	FeeBaseMsat      int64    `xml:"baseFee,attr"`
	FeeRateMilliMsat int64    `xml:"feeRate,attr"`
	Disabled         bool     `xml:"disabled,attr"`
}

func TestGenerateGraphGEXF(t *testing.T) () {
	client, ctx, err := Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	channelGraph, err := client.DescribeGraph(ctx, &lnrpc.ChannelGraphRequest{}, grpc.MaxCallRecvMsgSize(50*1024*1024))
	if err != nil {
		log.Fatalf("Could not get channel graph: %v", err)
	}

	log.Infof("%v", channelGraph.XXX_sizecache)

	graphNodes := make([]*GraphNode, len(channelGraph.Nodes))
	for i, node := range channelGraph.Nodes {
		graphNodes[i] = &GraphNode{
			ID:    node.PubKey,
			Label: node.Alias,
		}
	}

	graphEdges := make([]*GraphEdge, 2*len(channelGraph.Edges)+2)
	for i, edge := range channelGraph.Edges {
		if edge.Node1Policy == nil {
			log.Errorf("Channel %v has missing policy for node %v", edge.ChannelId, edge.Node1Pub)
		}

		if edge.Node2Policy == nil {
			log.Errorf("Channel %v has missing policy for node %v", edge.ChannelId, edge.Node2Pub)
		}

		if edge.Node1Policy != nil {
			graphEdges[i*2] = &GraphEdge{
				ID:               fmt.Sprintf("%v:%d", edge.Node1Pub, edge.ChannelId),
				Source:           edge.Node1Pub,
				Target:           edge.Node2Pub,
				FeeBaseMsat:      edge.Node1Policy.FeeBaseMsat,
				FeeRateMilliMsat: edge.Node1Policy.FeeRateMilliMsat,
				Disabled:         edge.Node1Policy.Disabled,
			}
		}

		if edge.Node2Policy != nil {
			graphEdges[i*2+1] = &GraphEdge{
				ID:               fmt.Sprintf("%v:%d", edge.Node2Pub, edge.ChannelId),
				Source:           edge.Node2Pub,
				Target:           edge.Node1Pub,
				FeeBaseMsat:      edge.Node2Policy.FeeBaseMsat,
				FeeRateMilliMsat: edge.Node2Policy.FeeRateMilliMsat,
				Disabled:         edge.Node2Policy.Disabled,
			}
		}
	}

	gexf := &GexfRoot{
		Xmlns:   "http://www.gexf.net/1.2draft",
		Version: "1.2",
		Graph: &GraphRoot{
			Mode:            "static",
			DefaultEdgeType: "directed",
			Nodes: &GraphNodes{
				Items: graphNodes,
			},
			Edges: &GraphEdges{
				Items: graphEdges,
			},
		},
	}

	filename := "graph.gexf"
	file, _ := os.Create(filename)

	xmlWriter := io.Writer(file)

	xmlWriter.Write([]byte(xml.Header))

	enc := xml.NewEncoder(xmlWriter)
	enc.Indent("  ", "    ")
	if err := enc.Encode(gexf); err != nil {
		log.Fatalf("Could not write graph data to GEXF file: %v", err)
	}

	log.Infof("Successfully written graph data to GEXF file")
}
