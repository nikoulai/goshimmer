package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

var (
	// only messages issued in the last timeWindow mins are taken into analysis
	timeWindow = -10 * time.Minute
	nodeInfos  = []*nodeInfo{
		{
			name:   "master",
			apiURL: "http://127.0.0.1:8080",
			mpm:    814,
			//mpm: 274,
		},
		{
			name:   "faucet",
			apiURL: "http://127.0.0.1:8090",
			mpm:    274,
			//mpm: 814,
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
	switch runSchedulerOrRateSetter() {
	case 'S':
	case 's':
		runScheduler()
		return
	case 'R':
	case 'r':
		runRateSetterFromScratch()
		return
	default:
		fmt.Println("invalid input")
		return
	}
}

func runSchedulerOrRateSetter() byte {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Run scheduler or rate setter? [S/R] ")

	text, _ := reader.ReadByte()
	return text
}
