package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"
)

var tableDescription = []string{
	"ID",
	"IssuerID",
	"ArrivalScheduledAvgDelay",
	"MinDelay",
	"AvgDelay",
	"MaxDelay",
}

func toCSVRow(nodeID, issuer string, delay schedulingInfo) []string {
	return []string{
		nodeID,
		issuer,
		fmt.Sprint(delay.arrivalScheduledAvgDelay),
		fmt.Sprint(delay.minDelay),
		fmt.Sprint(delay.avgDelay),
		fmt.Sprint(delay.maxDelay),
	}
}

func writeResultsToCSV(delayMaps map[string]map[string]schedulingInfo) {
	file, err := os.Create("schedulingDelay.csv")
	if err != nil {
		fmt.Println("open file is failed, err: ", err)
	}
	defer file.Close()

	csvWriter := csv.NewWriter(file)
	if err := csvWriter.Write(tableDescription); err != nil {
		fmt.Println("failed to write table description row: %w", err)
	}

	for nodeID, delays := range delayMaps {
		for issuer, delayInfo := range delays {
			row := toCSVRow(nodeID, issuer, delayInfo)
			if err := csvWriter.Write(row); err != nil {
				fmt.Println("failed to write message diagnostic info row: %w", err)
				return
			}
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		fmt.Println("csv writer failed after flush: %w", err)
	}
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

func printMinMaxAvg(delayMaps map[string]map[string]schedulingInfo) {
	fmt.Printf("The arrScheAvg/min/avg/max scheduling delay of different issuers on different nodes:\n\n")

	title := fmt.Sprintf("%-15s", "Issuer\\NodeID")
	for _, info := range nodeInfos {
		title = fmt.Sprintf("%s %-70s", title, info.name+"(arrScheAvg/min/avg/max)")
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
			delaystr := fmt.Sprintf("%v / %v / %v / %v",
				time.Duration(delayMaps[nodeID][issuerID].arrivalScheduledAvgDelay)*time.Nanosecond,
				time.Duration(delayMaps[nodeID][issuerID].minDelay)*time.Nanosecond,
				time.Duration(delayMaps[nodeID][issuerID].avgDelay)*time.Nanosecond,
				time.Duration(delayMaps[nodeID][issuerID].maxDelay)*time.Nanosecond)
			row = fmt.Sprintf("%s %-70s", row, delaystr)
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
