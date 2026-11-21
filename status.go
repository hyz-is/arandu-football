package football

import "strings"

// statusFromShort reduces the service's short status code to a [Status]. A code
// the package does not know is read as scheduled: a fixture is never reported
// finished or live on a guess.
func statusFromShort(short string) Status {
	switch strings.ToUpper(strings.TrimSpace(short)) {
	case "1H", "HT", "2H", "ET", "BT", "P", "LIVE", "INT":
		return StatusLive
	case "FT", "AET", "PEN", "AWD", "WO":
		return StatusFinished
	case "PST", "SUSP":
		return StatusPostponed
	case "CANC", "ABD":
		return StatusCancelled
	default: // NS, TBD and anything new
		return StatusScheduled
	}
}
