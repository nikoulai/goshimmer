package ratesettertracker

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

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

		row := toCSVRow(messageID, timestamp.Size, timestamp.SubmittedAt, timestamp.IssuedAt, timestamp.DiscardedAt)
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
}

func toCSVRow(messageID tangle.MessageID, size int, submittedAt, issuedAt, discardedAt time.Time) []string {
	return []string{
		messageID.Base58(),
		fmt.Sprint(submittedAt.Unix()),
		fmt.Sprint(issuedAt.Unix()),
		fmt.Sprint(discardedAt.Unix()),
		fmt.Sprint(size),
	}
}
