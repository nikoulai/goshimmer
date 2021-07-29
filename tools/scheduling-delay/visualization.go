package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

var xAxis = []string{}

func renderChart(nodeQSizes map[string]map[string][]nodeQueueSize,
	delayMaps map[string]map[string]schedulingInfo,
	rawDelay map[string]map[string][]time.Duration, manaPercentage map[string]float64) {
	// set xAxis
	for _, info := range nodeInfos {
		xAxis = append(xAxis, info.name)
	}

	page := components.NewPage()
	page.AddCharts(
		schedulingDelayLineChart(delayMaps, manaPercentage),
		schedulingDelayBoxPlot(rawDelay),
	)
	// nodeQ sizes charts are split by node
	nodeQCharts := nodeQSizeLineChart(nodeQSizes)
	for _, c := range nodeQCharts {
		page.AddCharts(c)
	}

	f, err := os.Create("./bar.html")
	if err != nil {
		panic(err)
	}
	page.Render(io.MultiWriter(f))
}

func schedulingDelayLineChart(delayMaps map[string]map[string]schedulingInfo, manaPercentage map[string]float64) *charts.Line {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Scheduling Delay of Each Issuer per Node"}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "NodeID",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:      "Avg Scheduling Delay",
			AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} ms"},
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true}),
		charts.WithLegendOpts(opts.Legend{
			Show:   true,
			Right:  "1%",
			Top:    "10%",
			Orient: "vertical",
		}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:  true,
			Right: "5%",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show:  true,
					Type:  "png",
					Title: "Anything you want",
				},
				DataView: &opts.ToolBoxFeatureDataView{
					Show:  true,
					Title: "DataView",
					// set the language
					// Chinese version: ["数据视图", "关闭", "刷新"]
					Lang: []string{"data view", "turn off", "refresh"},
				},
			},
		}),
	)
	line.SetXAxis(xAxis)

	lineItems := schedulingDelayLineItems(delayMaps)
	for nodeID, items := range lineItems {
		line.AddSeries(nodeID, items)
	}

	if manaPercentage != nil {
		line.Overlap(manaBarChart(manaPercentage))
	}

	return line
}

func schedulingDelayBoxPlot(rawData map[string]map[string][]time.Duration) *charts.BoxPlot {
	boxPlot := charts.NewBoxPlot()
	boxPlot.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Boxplot of scheduling delay"}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "nodeID",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:      "ms",
			Type:      "value",
			AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} ms"},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:  "inside",
			Start: 10,
			End:   50,
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true}),
		charts.WithLegendOpts(opts.Legend{
			Show:   true,
			Right:  "1%",
			Top:    "10%",
			Orient: "vertical",
		}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:  true,
			Right: "5%",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show:  true,
					Type:  "png",
					Title: "Download png file",
				},
				DataView: &opts.ToolBoxFeatureDataView{
					Show:  true,
					Title: "DataView",
					// set the language
					// Chinese version: ["数据视图", "关闭", "刷新"]
					Lang: []string{"data view", "turn off", "refresh"},
				},
			},
		}),
	)

	boxPlot.SetXAxis(xAxis)
	lineItems := schedulingDelayBoxPlotItems(rawData)
	for issuerID, items := range lineItems {
		boxPlot.AddSeries(issuerID, items)
	}

	return boxPlot
}

func schedulingDelayBoxPlotItems(rawData map[string]map[string][]time.Duration) map[string][]opts.BoxPlotData {
	items := make(map[string][]opts.BoxPlotData, len(xAxis))
	var issuersOrder []string
	for _, v := range rawData {
		for issuer := range v {
			issuersOrder = append(issuersOrder, issuer)
		}
		break
	}

	for _, issuerID := range issuersOrder {
		for _, node := range xAxis {
			nodeID := nameNodeInfoMap[node].nodeID
			var delays []int64
			for _, d := range rawData[nodeID][issuerID] {
				delays = append(delays, d.Milliseconds())
			}
			items[issuerID] = append(items[issuerID],
				opts.BoxPlotData{Value: delays})
		}
	}
	return items
}

func nodeQSizeLineChart(qSizes map[string]map[string][]nodeQueueSize) []*charts.Line {
	var lineCharts []*charts.Line
	var issuersOrder []string
	var node string
	for n, v := range qSizes {
		node = n
		for issuer := range v {
			issuersOrder = append(issuersOrder, issuer)
		}
		break
	}
	var nodeQXAxis []int
	for i := 0; i < len(qSizes[node][issuersOrder[0]]); i++ {
		nodeQXAxis = append(nodeQXAxis, i)
	}

	for nodeID, qsz := range qSizes {
		line := charts.NewLine()
		title := "The NodeQueue Size of Each Issuer on " + nodeID
		line.SetGlobalOptions(
			charts.WithTitleOpts(opts.Title{Title: title}),
			charts.WithXAxisOpts(opts.XAxis{
				Name: "time",
			}),
			charts.WithYAxisOpts(opts.YAxis{
				Name:      "buffered bytes in nodeQ",
				AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} bytes"},
			}),
			charts.WithDataZoomOpts(opts.DataZoom{
				Type:  "inside",
				Start: 10,
				End:   50,
			}),
			charts.WithTooltipOpts(opts.Tooltip{Show: true}),
			charts.WithLegendOpts(opts.Legend{
				Show:   true,
				Right:  "1%",
				Top:    "10%",
				Orient: "vertical",
			}),
			charts.WithToolboxOpts(opts.Toolbox{
				Show:  true,
				Right: "5%",
				Feature: &opts.ToolBoxFeature{
					SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
						Show:  true,
						Type:  "png",
						Title: "Download png file",
					},
					DataView: &opts.ToolBoxFeatureDataView{
						Show:  true,
						Title: "DataView",
						// set the language
						// Chinese version: ["数据视图", "关闭", "刷新"]
						Lang: []string{"data view", "turn off", "refresh"},
					},
				},
			}),
		)
		line.SetXAxis(nodeQXAxis)
		lineItems := nodeQueueSizeLineItems(issuersOrder, qsz)
		for issuerID, items := range lineItems {
			line.AddSeries(issuerID, items)
		}
		lineCharts = append(lineCharts, line)
	}

	return lineCharts
}

func schedulingDelayLineItems(delayMaps map[string]map[string]schedulingInfo) map[string][]opts.LineData {
	items := make(map[string][]opts.LineData, len(xAxis))
	var issuersOrder []string
	for _, v := range delayMaps {
		for issuer := range v {
			issuersOrder = append(issuersOrder, issuer)
		}
		break
	}

	for _, issuerID := range issuersOrder {
		for _, node := range xAxis {
			nodeID := nameNodeInfoMap[node].nodeID
			delay := time.Duration(delayMaps[nodeID][issuerID].avgDelay) * time.Nanosecond
			items[issuerID] = append(items[issuerID],
				opts.LineData{Value: delay.Milliseconds()})
		}
	}
	return items
}

func nodeQueueSizeLineItems(issuersOrder []string, nodeQSizes map[string][]nodeQueueSize) map[string][]opts.LineData {
	items := make(map[string][]opts.LineData, len(xAxis))

	for _, issuerID := range issuersOrder {
		for _, sz := range nodeQSizes[issuerID] {
			items[issuerID] = append(items[issuerID],
				opts.LineData{Value: sz.size})
		}
	}
	return items
}

func manaBarChart(manaMap map[string]float64) *charts.Bar {
	bar := charts.NewBar()
	items := []opts.BarData{}
	for _, issuer := range xAxis {
		issuerID := nameNodeInfoMap[issuer].nodeID
		mana, ok := manaMap[issuerID]
		if !ok {
			mana = 0
		}
		items = append(items, opts.BarData{Value: math.Round(mana * 100)})
	}

	bar.SetXAxis(xAxis).
		AddSeries("mana percentage", items).
		SetSeriesOptions(
			charts.WithLabelOpts(opts.Label{
				Show:      true,
				Position:  "insideBottom",
				Formatter: "{c} %",
			}),
		)
	return bar
}

func readFromCSVs() (nodeQSizes map[string]map[string][]nodeQueueSize,
	delayMaps map[string]map[string]schedulingInfo,
	rawDelay map[string]map[string][]time.Duration) {
	nodeQSizes = readNodeQSizesFromCSV("./nodeQueueSizes.csv")
	delayMaps = readSchedulingDelayFromCSV("./schedulingDelay.csv")
	rawDelay = readRawSchedulingDelayFromCSV("./schedulingDelayRawData.csv")

	return nodeQSizes, delayMaps, rawDelay
}

func readSchedulingDelayFromCSV(filePath string) (delayMaps map[string]map[string]schedulingInfo) {
	scheDelayFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println("cannot open" + filePath)
		return nil
	}

	// read scheduling delay from csv file
	scheDelayReader := csv.NewReader(scheDelayFile)
	delayMaps = make(map[string]map[string]schedulingInfo)
	// skip header
	if _, err := scheDelayReader.Read(); err != nil {
		panic(err)
	}
	records, err := scheDelayReader.ReadAll()
	if err != nil {
		fmt.Println(err)
	}

	for _, row := range records {
		if _, exist := delayMaps[row[0]]; !exist {
			delayMaps[row[0]] = make(map[string]schedulingInfo)
		}
		delayMaps[row[0]][row[1]] = schedulingInfo{
			arrivalScheduledAvgDelay: durationFromString(row[2]).Nanoseconds(),
			minDelay:                 durationFromString(row[3]).Nanoseconds(),
			avgDelay:                 durationFromString(row[4]).Nanoseconds(),
			maxDelay:                 durationFromString(row[5]).Nanoseconds(),
		}
	}

	return delayMaps
}

func readRawSchedulingDelayFromCSV(filePath string) (rawDelay map[string]map[string][]time.Duration) {
	rawScheDelayFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println("cannot open" + filePath)
		return nil
	}
	// read scheduling delay raw data from csv file
	rawScheDelayReader := csv.NewReader(rawScheDelayFile)
	rawDelay = make(map[string]map[string][]time.Duration)
	// skip header
	if _, err := rawScheDelayReader.Read(); err != nil {
		panic(err)
	}

	records, err := rawScheDelayReader.ReadAll()
	if err != nil {
		fmt.Println(err)
	}

	for _, row := range records {
		if _, exist := rawDelay[row[0]]; !exist {
			rawDelay[row[0]] = make(map[string][]time.Duration)
		}
		rawDelay[row[0]][row[1]] = append(rawDelay[row[0]][row[1]], durationFromString(row[2]))
	}

	return rawDelay
}

func readNodeQSizesFromCSV(filePath string) (nodeQSizes map[string]map[string][]nodeQueueSize) {
	nodeQFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println("cannot open" + filePath)
		return nil
	}
	// read nodeQueue sizes from csv file
	nodeQSizeReader := csv.NewReader(nodeQFile)
	nodeQSizes = make(map[string]map[string][]nodeQueueSize)
	// skip header
	if _, err := nodeQSizeReader.Read(); err != nil {
		panic(err)
	}

	records, err := nodeQSizeReader.ReadAll()
	if err != nil {
		fmt.Println(err)
	}

	for _, row := range records {
		if _, exist := nodeQSizes[row[0]]; !exist {
			nodeQSizes[row[0]] = make(map[string][]nodeQueueSize)
		}
		sz, _ := strconv.ParseInt(row[3], 10, 0)
		nodeQSizes[row[0]][row[1]] = append(nodeQSizes[row[0]][row[1]], nodeQueueSize{
			size:      int(sz),
			timestamp: timestampFromString(row[2]).UnixNano(),
		})
	}

	return nodeQSizes
}

func durationFromString(timeStr string) time.Duration {
	d, _ := strconv.ParseInt(timeStr, 10, 64)
	return time.Duration(d) * time.Nanosecond
}
