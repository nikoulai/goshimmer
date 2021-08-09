package client

import (
	"encoding/csv"
	"fmt"
	"net/http"
)

const (
	routeRateSetterTracker = "ratesettertracker"
)

// ToggleRateSetterTracker toggles the node ratesettertracker.
func (api *GoShimmerAPI) ToggleRateSetterTracker(enable bool) (*csv.Reader, error) {
	reader := &csv.Reader{}
	if err := api.do(http.MethodGet, func() string {
		if enable {
			return fmt.Sprintf("%s?cmd=start", routeRateSetterTracker)
		}
		return fmt.Sprintf("%s?cmd=stop", routeRateSetterTracker)
	}(), nil, reader); err != nil {
		fmt.Println(err)
		return nil, err
	}
	return reader, nil
}
