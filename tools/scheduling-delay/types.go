package main

import "github.com/iotaledger/goshimmer/client"

type mpsInfo struct {
	mps  float64
	msgs float64
}

type nodeQueueSize struct {
	size      int
	timestamp int64
}

type schedulingInfo struct {
	minDelay                 int64
	maxDelay                 int64
	avgDelay                 int64
	arrivalScheduledAvgDelay int64
	scheduledMsgs            int
	nodeQLen                 int
}

type nodeInfo struct {
	name   string
	apiURL string
	nodeID string
	client *client.GoShimmerAPI
	mpm    int
}
type backgroundAnalysisChan struct {
	shutdown chan struct{}
	//nodeQSizes chan map[string]map[string][]nodeQueueSize
}
