package message

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
	"github.com/iotaledger/goshimmer/plugins/webapi/message"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/labstack/echo"
)

// NextPastMarkerHandler is the handler for the /messages/:messageID endpoint.
func NextPastMarkerHandler(c echo.Context) (err error) {
	messageID, err := message.MessageIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	var count int
	messagelayer.Tangle().Utils.WalkMessageMetadata(func(messageMetadata *tangle.MessageMetadata, walk *walker.Walker) {
		structureDetails := messageMetadata.StructureDetails()
		if structureDetails == nil {
			return
		}

		if structureDetails.IsPastMarker {
			walk.StopWalk()
			fmt.Println(messageMetadata)
			return
		}

		if count%100 == 0 {
			fmt.Println("Count", count)
		}
		count++
		for _, approver := range messagelayer.Tangle().Utils.ApprovingMessageIDs(messageMetadata.ID(), tangle.StrongApprover) {
			walk.Push(approver)
		}
	}, tangle.MessageIDs{messageID})

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Message with %s", messageID)))
}
