package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"gonum.org/v1/plot/vg"

	"gonum.org/v1/plot/plotter"

	"gonum.org/v1/plot/plotutil"

	"gonum.org/v1/plot"
)

func runRateSetterFromScratch() {
	nameNodeInfoMap = make(map[string]*nodeInfo, len(nodeInfos))
	bindGoShimmerAPIAndNodeID()

	fmt.Println(time.Now())

	// start spamming
	toggleSpammer(true)
	toggleRateSetterTracker(true)

	// run background analysis: spammer, nodeQ size tracker
	bgAnalysisChan := &backgroundAnalysisChan{
		shutdown: make(chan struct{}),
	}
	runBackgroundAnalysis(bgAnalysisChan)

	time.Sleep(1 * time.Minute)

	// stop background analysis
	close(bgAnalysisChan.shutdown)

	// stop spamming
	toggleSpammer(false)
	csvs := toggleRateSetterTracker(false)
	result := parseCSV(csvs)
	writeResultToCSV(result)
	plotRateSetterDuration(result)
	plotRateSetterSize(result)
	plotRateSetterRate(result)

	discarded := int64(0)
	remained := int64(0)

	for _, v := range result {
		if v.DiscardedAt.Unix() == 0 {
			discarded++
		}
		if v.IssuedAt.Unix() == 0 {
			remained++
		}
	}

	fmt.Println("discarded: ", discarded)
	fmt.Println("remained: ", remained)
}

func toggleRateSetterTracker(enabled bool) (csvs []*csv.Reader) {
	info := nodeInfos[0]
	resp, err := info.client.ToggleRateSetterTracker(enabled)
	if err != nil {
		panic(err)
	}
	// debug logging
	if enabled {
		fmt.Println(info.name, "enables the ratesetter tracker", resp)
	} else {
		fmt.Println(info.name, "stops ratesetter tracker")
	}

	if resp != nil {
		csvs = append(csvs, resp)
	}

	return csvs
}

func parseCSV(csvs []*csv.Reader) (result map[string]messageTime) {
	result = make(map[string]messageTime)
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
			submittedAt, _ := strconv.ParseInt(row[1], 10, 64)
			issuedAt, _ := strconv.ParseInt(row[2], 10, 64)
			discardedAt, _ := strconv.ParseInt(row[3], 10, 64)
			size, _ := strconv.ParseInt(row[4], 10, 32)
			rate, _ := strconv.ParseFloat(row[5], 64)
			t := messageTime{
				SubmittedAt: time.Unix(0, int64(time.Millisecond)*submittedAt),
				IssuedAt:    time.Unix(0, int64(time.Millisecond)*issuedAt),
				DiscardedAt: time.Unix(0, int64(time.Millisecond)*discardedAt),
				Size:        int(size),
				Rate:        rate,
			}
			result[row[0]] = t
		}
	}
	return result
}

func plotRateSetterDuration(result map[string]messageTime) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = "Time spent in rate setter queue"
	p.Y.Label.Text = "Duration (ms)"
	p.X.Label.Text = "Message tick"

	pts := make(plotter.XYs, len(result))
	i := int64(0)
	for _, v := range result {
		pts[i].X = float64(i)
		diff := v.IssuedAt.Sub(v.SubmittedAt)
		if diff.Milliseconds() < 0 {
			continue
		}
		pts[i].Y = float64(diff.Milliseconds())
		i++
	}
	err := plotutil.AddLinePoints(p, "duration", pts)
	if err != nil {
		panic(err)
	}
	if err := p.Save(9*vg.Inch, 9*vg.Inch, "duration.png"); err != nil {
		panic(err)
	}
}

func plotRateSetterSize(result map[string]messageTime) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = "The size of the rate setter queue"
	p.Y.Label.Text = "Size (bytes)"
	p.X.Label.Text = "Message tick"

	pts := make(plotter.XYs, len(result))
	i := int64(0)
	for _, v := range result {
		pts[i].X = float64(i)
		pts[i].Y = float64(v.Size)
		i++
	}
	err := plotutil.AddLinePoints(p, "size", pts)
	if err != nil {
		panic(err)
	}
	if err := p.Save(9*vg.Inch, 9*vg.Inch, "size.png"); err != nil {
		panic(err)
	}
}

func plotRateSetterRate(result map[string]messageTime) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = "Rate of rate setter queue"
	p.Y.Label.Text = "Rate (bytes/sec)"
	p.X.Label.Text = "Message tick"

	pts := make(plotter.XYs, len(result))
	i := int64(0)
	for _, v := range result {
		pts[i].X = float64(i)
		pts[i].Y = v.Rate
		i++
	}
	err := plotutil.AddLinePoints(p, "rate", pts)
	if err != nil {
		panic(err)
	}
	if err := p.Save(9*vg.Inch, 9*vg.Inch, "rate.png"); err != nil {
		panic(err)
	}
}

var tableDesc = []string{
	"submittedAt",
	"issuedAt",
	"duration",
	"discardedAt",
	"duration",
	"size",
	"rate",
}

func toCSVRow(submittedAt, issuedAt, discardedAt time.Time, size int, rate float64) []string {
	diff := issuedAt.Sub(submittedAt)
	if diff.Milliseconds() < 0 {
		diff = time.Second * -10
	}
	return []string{
		fmt.Sprint(submittedAt.Unix()),
		fmt.Sprint(issuedAt.Unix()),
		fmt.Sprint(diff.String()),
		fmt.Sprint(discardedAt.Unix()),
		fmt.Sprint(discardedAt.Sub(submittedAt).String()),
		fmt.Sprint(size),
		fmt.Sprint(rate),
	}
}

func writeResultToCSV(result map[string]messageTime) {
	file, err := os.Create("ratesetter.csv")
	if err != nil {
		fmt.Println("open file is failed, err: ", err)
	}
	defer file.Close()

	csvWriter := csv.NewWriter(file)
	if err := csvWriter.Write(tableDesc); err != nil {
		fmt.Println("failed to write table description row: %w", err)
	}

	for _, v := range result {
		row := toCSVRow(v.SubmittedAt, v.IssuedAt, v.DiscardedAt, v.Size, v.Rate)
		if err := csvWriter.Write(row); err != nil {
			fmt.Println("failed to write message diagnostic info row: %w", err)
			return
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		fmt.Println("csv writer failed after flush: %w", err)
	}
}
