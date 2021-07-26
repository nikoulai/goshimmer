package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/client"
)

var (
	// only messages issued in the last timeWindow mins are taken into analysis
	timeWindow      = -10 * time.Minute
	nodeInfos       []*nodeInfo
	nameNodeInfoMap map[string]*nodeInfo
)

var (
	seeds = []string{
		"CDDzcUNbok6zyoF8zC8gpD2E2pjGdBEm2Lkpauc3PSGk",
		"7RcW1L4xfUXCyubnYxSeJ3XWMfhXyAJMBDppQUmQAo6z",
		"2j9tYwGkannQ92FPZ5uwn6eutcQaJDDvuEDFZNESGQxz",
		"AzZ4wGrPqgP5mbZLGQc9onKzsJ2NJvtjLQQ9Bkrins87",
		// "BBew186Ms89jqaAyuVANuhkoR9wu37o1nZ36K5NztDze",
	}

	nine = []string{
		"BBew186Ms89jqaAyuVANuhkoR9wu37o1nZ36K5NztDze",
	}
)

type nodeInfo struct {
	name   string
	apiURL string
	nodeID string
	client *client.GoShimmerAPI
	mpm    int
}

type mpsInfo struct {
	mps  float64
	msgs float64
}

type schedulingInfo struct {
	minDelay                 int64
	maxDelay                 int64
	avgDelay                 int64
	arrivalScheduledAvgDelay int64
	scheduledMsgs            int
	nodeQLen                 int
}

func main() {
	nodeInfos = []*nodeInfo{
		{
			name:   "master",
			apiURL: "http://127.0.0.1:8080",
			mpm:    814,
		},
		{
			name:   "faucet",
			apiURL: "http://127.0.0.1:8090",
			mpm:    274,
		},
	}
	nameNodeInfoMap = make(map[string]*nodeInfo, len(nodeInfos))
	bindGoShimmerAPIAndNodeID()

	fmt.Println(time.Now())

	// start spamming
	toggleSpammer(true)
	spamWithNodes(nameNodeInfoMap["faucet"].client)

	// time.Sleep(11 * time.Minute)

	// start collecting metrics
	endTime := time.Now()
	delayMaps := make(map[string]map[string]schedulingInfo, len(nodeInfos))
	mpsMaps := make(map[string]map[string]mpsInfo, len(nodeInfos))
	for _, info := range nodeInfos {
		apiInfo, err := info.client.Info()
		if err != nil {
			fmt.Println(info.apiURL, "crashed")
			continue
		}
		delayMaps[info.nodeID] = analyzeSchedulingDelay(info.client, endTime)
		mpsMaps[info.nodeID] = analyzeMPSDistribution(info.client, endTime)
		// get node queue sizes
		for issuer, qLen := range apiInfo.Scheduler.NodeQueueSizes {
			t := delayMaps[info.nodeID][issuer]
			t.nodeQLen = qLen
			delayMaps[info.nodeID][issuer] = t
		}
	}

	// stop spamming
	toggleSpammer(false)

	printResults(delayMaps)
	printMinMaxAvg(delayMaps)
	printMPSResults(mpsMaps)
	printStoredMsgsPercentage(mpsMaps)

	manaPercentage := fetchManaPercentage(nodeInfos[0].client)
	renderChart(delayMaps, manaPercentage)
}

func bindGoShimmerAPIAndNodeID() {
	for _, info := range nodeInfos {
		// create GoShimmer API
		api := client.NewGoShimmerAPI(info.apiURL, client.WithHTTPClient(http.Client{Timeout: 1800 * time.Second}))
		// get short node ID
		nodeInfo, err := api.Info()
		if err != nil {
			fmt.Println(api.BaseURL(), "crashed")
			continue
		}
		info.nodeID = nodeInfo.IdentityIDShort
		info.client = api

		nameNodeInfoMap[info.name] = info
	}
}

func toggleSpammer(enabled bool) {
	for _, info := range nodeInfos {
		if info.mpm <= 0 {
			continue
		}

		resp, err := info.client.ToggleSpammer(enabled, info.mpm)
		if err != nil {
			panic(err)
		}
		// debug logging
		if enabled {
			fmt.Println(info.name, "spamming at", info.mpm, resp)
		} else {
			fmt.Println(info.name, "stop spamming")
		}
	}
}

func analyzeSchedulingDelay(goshimmerAPI *client.GoShimmerAPI, endTime time.Time) map[string]schedulingInfo {
	csvRes, err := goshimmerAPI.GetDiagnosticsMessages()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	messageInfos, _ := csvRes.ReadAll()

	scheduleDelays := calculateSchedulingDelay(messageInfos, endTime)
	arrivalScheduledDelays := calculateArrivalScheduledDelay(messageInfos, endTime)

	// the average of delay per node
	avgScheduleDelay := make(map[string]schedulingInfo)
	for nodeID, delays := range scheduleDelays {
		var bookedScheduledSum, arrivalScheduledSum int64 = 0, 0
		var max, min int64 = 0, math.MaxInt64

		// arrival ~ scheduled
		for _, d := range arrivalScheduledDelays[nodeID] {
			arrivalScheduledSum += d.Nanoseconds()
		}

		// booked ~ scheduled
		for _, d := range delays {
			bookedScheduledSum += d.Nanoseconds()
			if d.Nanoseconds() < min {
				min = d.Nanoseconds()
			}
			if d.Nanoseconds() > max {
				max = d.Nanoseconds()
			}
		}
		avgScheduleDelay[nodeID] = schedulingInfo{
			minDelay:                 min,
			maxDelay:                 max,
			avgDelay:                 bookedScheduledSum / int64(len(delays)),
			arrivalScheduledAvgDelay: arrivalScheduledSum / int64(len(arrivalScheduledDelays[nodeID])),
			scheduledMsgs:            len(delays),
		}
	}

	return avgScheduleDelay
}

func analyzeMPSDistribution(goshimmerAPI *client.GoShimmerAPI, endTime time.Time) map[string]mpsInfo {
	csvRes, err := goshimmerAPI.GetDiagnosticsMessages()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return calculateMPS(csvRes, endTime)
}

func calculateSchedulingDelay(messageInfos [][]string, endTime time.Time) map[string][]time.Duration {
	startTime := endTime.Add(timeWindow)
	nodeDelayMap := make(map[string][]time.Duration)

	for _, msg := range messageInfos {
		bookedTime := timestampFromString(msg[7])
		// ignore data that is issued before collectTime
		if bookedTime.Before(startTime) || bookedTime.After(endTime) {
			continue
		}

		scheduledTime := timestampFromString(msg[6])
		// ignore if the message is not yet scheduled
		if scheduledTime.Before(startTime) || scheduledTime.After(endTime) {
			continue
		}

		issuer := msg[1]
		nodeDelayMap[issuer] = append(nodeDelayMap[issuer], scheduledTime.Sub(bookedTime))
	}
	return nodeDelayMap
}

func calculateArrivalScheduledDelay(messageInfos [][]string, endTime time.Time) map[string][]time.Duration {
	startTime := endTime.Add(timeWindow)
	nodeDelayMap := make(map[string][]time.Duration)

	for _, msg := range messageInfos {
		arrivalTime := timestampFromString(msg[4])
		// ignore data that is issued before collectTime
		if arrivalTime.Before(startTime) || arrivalTime.After(endTime) {
			continue
		}

		scheduledTime := timestampFromString(msg[6])
		// ignore if the message is not yet scheduled
		if scheduledTime.Before(startTime) || scheduledTime.After(endTime) {
			continue
		}

		issuer := msg[1]
		nodeDelayMap[issuer] = append(nodeDelayMap[issuer], scheduledTime.Sub(arrivalTime))
	}
	return nodeDelayMap
}

func calculateMPS(response *csv.Reader, endTime time.Time) map[string]mpsInfo {
	startTime := endTime.Add(timeWindow)
	nodeMSGCounterMap := make(map[string]int)
	nodeMPSMap := make(map[string]mpsInfo)
	messageInfos, _ := response.ReadAll()
	totalMsgFromStart := 0
	storedMsgFromStart := make(map[string]int)

	for _, msg := range messageInfos {
		issuer := msg[1]
		totalMsgFromStart++
		storedMsgFromStart[issuer]++

		arrivalTime := timestampFromString(msg[4])
		// ignore data that is issued before collectTime
		if arrivalTime.Before(startTime) || arrivalTime.After(endTime) {
			continue
		}

		nodeMSGCounterMap[issuer]++
	}

	for nodeID, counter := range nodeMSGCounterMap {
		nodeMPSMap[nodeID] = mpsInfo{
			mps:  float64(counter) / endTime.Sub(startTime).Seconds(),
			msgs: float64(storedMsgFromStart[nodeID]) / float64(totalMsgFromStart),
		}
	}
	return nodeMPSMap
}

func fetchManaPercentage(goshimmerAPI *client.GoShimmerAPI) map[string]float64 {
	manaPercentageMap := make(map[string]float64)
	res, _ := goshimmerAPI.GetNHighestAccessMana(0)

	totalAccessMana := 0.0
	for _, node := range res.Nodes {
		totalAccessMana += node.Mana
	}

	for _, node := range res.Nodes {
		manaPercentageMap[node.ShortNodeID] = node.Mana / totalAccessMana
	}
	return manaPercentageMap
}

func timestampFromString(timeString string) time.Time {
	timeInt64, _ := strconv.ParseInt(timeString, 10, 64)
	return time.Unix(0, timeInt64)
}

func printResults(delayMaps map[string]map[string]schedulingInfo) {
	fmt.Printf("The average scheduling delay of different issuers on different nodes:\n\n")

	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for _, info := range nodeInfos {
		title = fmt.Sprintf("%s %-30s %-15s", title, info.name, "scheduled msgs")
	}
	fmt.Printf("%s\n\n", title)

	var issuers map[string]schedulingInfo
	for _, v := range delayMaps {
		issuers = v
		break
	}

	for issuerID := range issuers {
		row := fmt.Sprintf("%-15s", issuerID)
		// issuerID := issuer.nodeID
		for _, node := range nodeInfos {
			nodeID := node.nodeID
			delayQLenstr := fmt.Sprintf("%v (Q size:%d)",
				time.Duration(delayMaps[nodeID][issuerID].avgDelay)*time.Nanosecond,
				delayMaps[nodeID][issuerID].nodeQLen)
			row = fmt.Sprintf("%s %-30s %-15d", row, delayQLenstr, delayMaps[nodeID][issuerID].scheduledMsgs)
		}
		fmt.Println(row)
	}
	fmt.Printf("\n")
}

func printMPSResults(mpsMaps map[string]map[string]mpsInfo) {
	fmt.Printf("The average mps of different issuers on different nodes:\n\n")

	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for _, info := range nodeInfos {
		title = fmt.Sprintf("%s %-30s", title, info.name)
	}
	fmt.Printf("%s\n\n", title)

	var issuers map[string]mpsInfo
	for _, v := range mpsMaps {
		issuers = v
		break
	}

	for issuerID := range issuers {
		row := fmt.Sprintf("%-15s", issuerID)
		for _, node := range nodeInfos {
			row = fmt.Sprintf("%s %-30f", row, mpsMaps[node.nodeID][issuerID].mps)
		}
		fmt.Println(row)
	}
	fmt.Printf("\n")
}

func printStoredMsgsPercentage(mpsMaps map[string]map[string]mpsInfo) {
	fmt.Printf("The proportion of msgs from different issuers on different nodes:\n\n")

	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for _, info := range nodeInfos {
		title = fmt.Sprintf("%s %-30s", title, info.name)
	}
	fmt.Printf("%s\n\n", title)

	var issuers map[string]mpsInfo
	for _, v := range mpsMaps {
		issuers = v
		break
	}

	for issuerID := range issuers {
		row := fmt.Sprintf("%-15s", issuerID)
		for _, node := range nodeInfos {
			row = fmt.Sprintf("%s %-30f", row, mpsMaps[node.nodeID][issuerID].msgs)
		}
		fmt.Println(row)
	}
}

func spam(api *client.GoShimmerAPI, pk ed25519.PrivateKey, rate time.Duration, shutdown chan struct{}) {
	ticker := time.NewTicker(rate)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			msgID, err := api.Data(pk.Public().Bytes(), pk.Public(), pk)
			fmt.Println(msgID, err)
		case <-shutdown:
			return
		}
	}
}

func spamWithNodes(api *client.GoShimmerAPI) {
	nodes := make(map[string]*identity.LocalIdentity)
	// api := client.NewGoShimmerAPI(apiURL)
	shutdown := make(chan struct{})
	var wg sync.WaitGroup
	for _, seed := range seeds {
		s, _ := base58.Decode(seed)
		pk := ed25519.PrivateKeyFromSeed(s[:])
		nodeIdentity := identity.NewLocalIdentity(pk.Public(), pk)
		fmt.Println(base58.Encode(nodeIdentity.ID().Bytes()))
		nodes[nodeIdentity.ID().String()] = nodeIdentity
		wg.Add(1)

		go func() {
			defer wg.Done()
			randomizedStart := rand.Intn(5000)
			time.Sleep(time.Duration(randomizedStart) * time.Millisecond)
			spam(api, pk, 5*time.Second, shutdown)
		}()
	}

	for _, seed := range nine {
		s, _ := base58.Decode(seed)
		pk := ed25519.PrivateKeyFromSeed(s[:])
		nodeIdentity := identity.NewLocalIdentity(pk.Public(), pk)
		fmt.Println(base58.Encode(nodeIdentity.ID().Bytes()))
		nodes[nodeIdentity.ID().String()] = nodeIdentity
		wg.Add(1)

		go func() {
			defer wg.Done()
			randomizedStart := rand.Intn(5000)
			time.Sleep(time.Duration(randomizedStart) * time.Millisecond)
			spam(api, pk, 5556*time.Millisecond, shutdown)
		}()
	}

	for i := 0; i < 1; i++ {
		b := make([]byte, 32)
		_, err := rand.Read(b)
		if err != nil {
			fmt.Println(err)
			return
		}
		pk := ed25519.PrivateKeyFromSeed(b[:])
		nodeIdentity := identity.NewLocalIdentity(pk.Public(), pk)
		fmt.Println(base58.Encode(nodeIdentity.ID().Bytes()))
		nodes[nodeIdentity.ID().String()] = nodeIdentity
		wg.Add(1)

		go func() {
			defer wg.Done()
			randomizedStart := rand.Intn(5000)
			time.Sleep(time.Duration(randomizedStart) * time.Millisecond)
			spam(api, pk, 72*time.Second, shutdown)
		}()
	}

	time.Sleep(11 * time.Minute)
	close(shutdown)
	wg.Wait()
}
