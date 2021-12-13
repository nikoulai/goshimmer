package nodeqsizetracker

import (
	"encoding/csv"
	"net/http"
	"sort"

	"github.com/labstack/echo"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

func handleRequest(c echo.Context) error {
	var request jsonmodels.NodeQTrackerRequest
	if err := c.Bind(&request); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	c.Response().Header().Set(echo.HeaderContentType, "text/csv")

	switch request.Cmd {
	case "start":
		stop()
		start()
		log.Infof("Started tracking nodeQ sizes")
		return c.NoContent(http.StatusOK)
	case "stop":
		stop()
		log.Info("Stopped tracking nodeQ sizes")
		return sendCSVResults(c)
	default:
		return c.NoContent(http.StatusBadRequest)
	}
}

func sendCSVResults(c echo.Context) error {
	// write Header and table description
	c.Response().WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(c.Response())
	if err := csvWriter.Write(nodeQSizeTableDescription); err != nil {
		return errors.Errorf("failed to write table description row: %s", err)
	}

	nodeID := deps.Tangle.Options.Identity.ID().String()
	var keys []int64
	for k := range nodeQSizeMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for _, timestamp := range keys {
		for issuer, sz := range nodeQSizeMap[timestamp] {
			row := nodeQToCSVRow(nodeID, issuer.String(), timestamp, sz)
			if err := csvWriter.Write(row); err != nil {
				log.Errorf("failed to write message diagnostic info row: %s", err)
			}
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return errors.Errorf("csv writer failed after flush: %s", err)
	}
	return nil
}
