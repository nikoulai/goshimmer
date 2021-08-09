package ratesettertracker

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func handleRequest(c echo.Context) error {
	var request jsonmodels.RateSetterTrackerRequest
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")

	switch request.Cmd {
	case "start":
		stop()
		start()
		log.Infof("Started tracking ratesetter")
		return c.NoContent(http.StatusOK)
	case "stop":
		stop()
		log.Info("Stopped tracking ratesetter")
		return sendCSVResults(c)
	default:
		return c.NoContent(http.StatusBadRequest)
	}
}

func sendCSVResults(c echo.Context) error {
	// write Header and table description
	c.Response().WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(c.Response())
	if err := csvWriter.Write(tableDes); err != nil {
		return errors.Errorf("failed to write table description row: %w", err)
	}

	for messageID, timestamp := range messageTimeMap {
		row := toCSVRow(messageID, timestamp.Size, timestamp.SubmittedAt, timestamp.IssuedAt, timestamp.DiscardedAt, timestamp.Rate)
		if err := csvWriter.Write(row); err != nil {
			log.Errorf("failed to write message diagnostic info row: %w", err)
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return errors.Errorf("csv writer failed after flush: %w", err)
	}
	return nil
}

var tableDes = []string{
	"MessageID",
	"SubmittedAt",
	"IssuedAt",
	"DiscardedAt",
	"Size",
	"Rate",
}

func toCSVRow(messageID tangle.MessageID, size int, submittedAt, issuedAt, discardedAt int64, rate float64) []string {
	return []string{
		messageID.Base58(),
		fmt.Sprint(submittedAt),
		fmt.Sprint(issuedAt),
		fmt.Sprint(discardedAt),
		fmt.Sprint(size),
		fmt.Sprint(rate),
	}
}
