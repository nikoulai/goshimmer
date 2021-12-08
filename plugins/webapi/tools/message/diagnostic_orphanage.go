package message

import (
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/echo"
	"net/http"
	"strconv"
	"time"
)

func DiagnosticOrphanageHandler(c echo.Context) error {
	return diagnosticOrphanage(c)
}

func diagnosticOrphanage(c echo.Context) error {
	ownId := deps.Local.ID().String() // TODO is this short nodeID?
	orphanCounts := make(map[string]int)
	issuedCount := make(map[string]int)

	requestStartTime, err := timeFromContext(c)
	if err != nil {
		requestStartTime = time.Now()
	}

	maxAge := deps.Tangle.Options.SolidifierParams.MaxParentsTimeDifference

	deps.Tangle.Utils.WalkMessageID(func(msgID tangle.MessageID, walker *walker.Walker) {
		// we assume no conflicts
		approverMessageIDs := deps.Tangle.Utils.ApprovingMessageIDs(msgID)
		var timestamp time.Time
		var notConfirmed bool
		var issuer string

		deps.Tangle.Storage.Message(msgID).Consume(func(message *tangle.Message) {
			timestamp = message.IssuingTime()
			pubKey := message.IssuerPublicKey()
			issuer = identity.New(pubKey).ID().String()

			deps.Tangle.Storage.MessageMetadata(msgID).Consume(func(messageMetadata *tangle.MessageMetadata) {
				// received before max parent age
				grade := messageMetadata.GradeOfFinality()
				notConfirmed = grade != gof.High
			})
		})
		// count only messages older than parent age check from the API response collection start
		if timestamp.Add(maxAge).Before(requestStartTime) {
			if _, ok := issuedCount[issuer]; !ok {
				issuedCount[issuer] = 0
			}
			issuedCount[issuer]++

			// message has no parents - is orphaned
			if len(approverMessageIDs) == 0 {
				if _, ok := orphanCounts[issuer]; !ok {
					orphanCounts[issuer] = 0
				}
				orphanCounts[issuer]++
			}
		}
		// continue walking
		for _, approverMessageID := range approverMessageIDs {
			walker.Push(approverMessageID)
		}
	}, tangle.MessageIDs{tangle.EmptyMessageID})

	return c.JSON(http.StatusOK, jsonmodels.NewOrphanageResponse(ownId, requestStartTime, maxAge, issuedCount, orphanCounts))
}

type DiagnosticOrphanage struct {
	orphansCount   int
	orphanedMsgIDs tangle.MessageIDs
}

// messageIDFromContext determines the MessageID from the messageID parameter in an echo.Context. It expects it to
// either be a base58 encoded string or the builtin alias EmptyMessageID.
func timeFromContext(c echo.Context) (requestTime time.Time, err error) {
	requestTimeStr := c.Param("time")
	requestInt, err := strconv.Atoi(requestTimeStr)
	if err != nil {
		return
	}
	requestTime = time.Unix(0, int64(requestInt))
	return
}
