package client

import (
	"encoding/csv"
	"fmt"
	"net/http"
)

const (
	routeNodeQSizeTracker = "nodeqsizetracker"
)

// ToggleNodeQSizeTracker toggles the node internal nodeQ size tracker.
func (api *GoShimmerAPI) ToggleNodeQSizeTracker(enable bool) (*csv.Reader, error) {
	reader := &csv.Reader{}
	if err := api.do(http.MethodGet, func() string {
		if enable {
			return fmt.Sprintf("%s?cmd=start", routeNodeQSizeTracker)
		}
		return fmt.Sprintf("%s?cmd=stop", routeNodeQSizeTracker)
	}(), nil, reader); err != nil {
		fmt.Println(err)
		return nil, err
	}
	return reader, nil
}
