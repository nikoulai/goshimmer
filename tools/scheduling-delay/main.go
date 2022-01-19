package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/client"
)

var (
	// only messages issued in the last timeWindow mins are taken into analysis
	timeWindow = -10 * time.Minute
	nodeInfos  = []*nodeInfo{
		{
			name:   "master",
			apiURL: "http://127.0.0.1:8080",
			mpm:    8340,
		},
		{
			name:   "faucet",
			apiURL: "http://127.0.0.1:8090",
			mpm:    3000,
		},
		{
			name:   "master2",
			apiURL: "http://127.0.0.1:8070",
			mpm:    0,
		},
	}
	nameNodeInfoMap        map[string]*nodeInfo
	schedulingDelayRawData map[string]map[string][]time.Duration
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

func main() {
	run := renderChartOrRun()
	if run {
		runFromScratch()
	} else {
		renderChartsFromCSVs()
	}
}

func renderChartOrRun() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("run the analysis from scratch? [Y/N] ")

	text, _ := reader.ReadByte()
	return text == 'Y'
}

func renderChartsFromCSVs() {
	nodeQSizes, delayMaps, schedulingDelayRaw := readFromCSVs()
	// Improve this if possible :P
	nameNodeInfoMap = make(map[string]*nodeInfo, len(nodeInfos))
	nodeInfos[1].nodeID = "dAnF7pQ6k7a"
	nodeInfos[0].nodeID = "4AeXyZ26e4G"
	nameNodeInfoMap["faucet"] = nodeInfos[1]
	nameNodeInfoMap["master"] = nodeInfos[0]

	printResults(delayMaps)
	printMinMaxAvg(delayMaps)

	renderChart(nodeQSizes, delayMaps, schedulingDelayRaw, nil)
}

func runFromScratch() {
	nameNodeInfoMap = make(map[string]*nodeInfo, len(nodeInfos))
	bindGoShimmerAPIAndNodeID()

	fmt.Println(time.Now())

	// start spamming
	toggleSpammer(true)
	toggleNodeQTracker(true)

	// run background analysis: spammer, nodeQ size tracker
	bgAnalysisChan := &backgroundAnalysisChan{
		shutdown: make(chan struct{}),
		//nodeQSizes: make(chan map[string]map[string][]nodeQueueSize),
	}
	runBackgroundAnalysis(bgAnalysisChan)

	time.Sleep(11 * time.Minute)

	// stop background analysis
	close(bgAnalysisChan.shutdown)
	//nodeQSizes := <-bgAnalysisChan.nodeQSizes

	// start collecting metrics
	endTime := time.Now()
	schedulingDelayRawData = make(map[string]map[string][]time.Duration)
	delayMaps := make(map[string]map[string]schedulingInfo, len(nodeInfos))
	mpsMaps := make(map[string]map[string]mpsInfo, len(nodeInfos))
	for _, info := range nodeInfos {
		apiInfo, err := info.client.Info()
		if err != nil {
			fmt.Println(info.apiURL, "crashed")
			continue
		}
		delayMaps[info.nodeID], schedulingDelayRawData[info.nodeID] = analyzeSchedulingDelay(info.client, endTime)
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
	nodeQCSVs := toggleNodeQTracker(false)
	nodeQSizes := processNodeQCSVS(nodeQCSVs)

	printResults(delayMaps)
	printMinMaxAvg(delayMaps)
	printMPSResults(mpsMaps)
	printStoredMsgsPercentage(mpsMaps)

	writeDelayResultsToCSV(delayMaps)
	writeDelayRawDataToCSV(schedulingDelayRawData)
	writeNodeQueueSizesToCSV(nodeQSizes)

	manaPercentage := fetchManaPercentage(nodeInfos[0].client)
	renderChart(nodeQSizes, delayMaps, schedulingDelayRawData, manaPercentage)
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

func toggleNodeQTracker(enabled bool) (nodeQCSVs []*csv.Reader) {
	for _, info := range nodeInfos {
		resp, err := info.client.ToggleNodeQSizeTracker(enabled)
		if err != nil {
			panic(err)
		}
		// debug logging
		if enabled {
			fmt.Println(info.name, "enables nodeQ size tracker", resp)
		} else {
			fmt.Println(info.name, "stops nodeQ size tracker")
		}

		if resp != nil {
			nodeQCSVs = append(nodeQCSVs, resp)
		}
	}
	return nodeQCSVs
}

func toggleSpammer(enabled bool) {
	for _, info := range nodeInfos {
		if info.mpm <= 0 {
			continue
		}

		resp, err := info.client.ToggleSpammer(enabled, info.mpm, "mpm", "uniform")
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

func runBackgroundAnalysis(bgChans *backgroundAnalysisChan) {
	spamWithNodes(nameNodeInfoMap["faucet"].client, bgChans.shutdown)
	//getNodeQueueSizes(nodeInfos, bgChans.shutdown, bgChans.nodeQSizes)
}

func analyzeSchedulingDelay(goshimmerAPI *client.GoShimmerAPI, endTime time.Time) (map[string]schedulingInfo, map[string][]time.Duration) {
	csvRes, err := goshimmerAPI.GetDiagnosticsMessages()
	if err != nil {
		fmt.Println(err)
		return nil, nil
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

	return avgScheduleDelay, scheduleDelays
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

func spamWithNodes(api *client.GoShimmerAPI, shutdown chan struct{}) {
	nodes := make(map[string]*identity.LocalIdentity)
	// api := client.NewGoShimmerAPI(apiURL)
	for _, seed := range seeds {
		s, _ := base58.Decode(seed)
		pk := ed25519.PrivateKeyFromSeed(s[:])
		nodeIdentity := identity.NewLocalIdentity(pk.Public(), pk)
		fmt.Println(base58.Encode(nodeIdentity.ID().Bytes()))
		nodes[nodeIdentity.ID().String()] = nodeIdentity

		go func() {
			randomizedStart := rand.Intn(5000)
			time.Sleep(time.Duration(randomizedStart) * time.Millisecond)
			spam(api, pk, 500*time.Millisecond, shutdown)
		}()
	}

	for _, seed := range nine {
		s, _ := base58.Decode(seed)
		pk := ed25519.PrivateKeyFromSeed(s[:])
		nodeIdentity := identity.NewLocalIdentity(pk.Public(), pk)
		fmt.Println(base58.Encode(nodeIdentity.ID().Bytes()))
		nodes[nodeIdentity.ID().String()] = nodeIdentity

		go func() {
			randomizedStart := rand.Intn(5000)
			time.Sleep(time.Duration(randomizedStart) * time.Millisecond)
			spam(api, pk, 556*time.Millisecond, shutdown)
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

		go func() {
			randomizedStart := rand.Intn(5000)
			time.Sleep(time.Duration(randomizedStart) * time.Millisecond)
			spam(api, pk, 72*time.Second, shutdown)
		}()
	}
}

func getNodeQueueSizes(apis []*nodeInfo, shutdown chan struct{}, sendResult chan map[string]map[string][]nodeQueueSize) {
	qSizes := make(map[string]map[string][]nodeQueueSize, len(apis))

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for _, api := range apis {
					now := time.Now().UnixNano()
					info, err := api.client.Info()
					if err != nil {
						continue
					}
					// get node queue sizes
					for issuer, qLen := range info.Scheduler.NodeQueueSizes {
						if qSizes[api.nodeID] == nil {
							qSizes[api.nodeID] = make(map[string][]nodeQueueSize)
						}

						t := qSizes[api.nodeID][issuer]
						t = append(t, nodeQueueSize{
							size:      qLen,
							timestamp: now,
						})
						qSizes[api.nodeID][issuer] = t
					}
				}
			case <-shutdown:
				sendResult <- qSizes
				return
			}
		}
	}()
}

func processNodeQCSVS(csvs []*csv.Reader) (result map[string]map[string][]nodeQueueSize) {
	result = make(map[string]map[string][]nodeQueueSize)
	for _, reader := range csvs {
		// skip header
		if _, err := reader.Read(); err != nil {
			panic(err)
		}
		records, err := reader.ReadAll()
		if err != nil {
			fmt.Println(err)
		}

		for _, row := range records {
			nodeID, issuerID := row[0], row[1]
			if result[nodeID] == nil {
				result[nodeID] = make(map[string][]nodeQueueSize)
			}

			t := result[nodeID][issuerID]
			ts, _ := strconv.ParseInt(row[2], 10, 64)
			sz, _ := strconv.ParseInt(row[3], 10, 0)
			t = append(t, nodeQueueSize{
				size:      int(sz),
				timestamp: ts,
			})
			result[nodeID][issuerID] = t
		}
	}
	return result
}
