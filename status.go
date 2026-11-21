package football

import "strings"

// statusFromShort reduces the service's short status code to a [Status]. SUSP
// and INT are "In Play" in the service's status table, so both are live. AWD and
// WO are "Not Played" there: the result was awarded, so they are finished. ABD
// (abandoned) may or may not be rescheduled and is read as cancelled. A code the
// package does not know is read as scheduled: a fixture is never reported
// finished or live on a guess.
func statusFromShort(short string) Status {
	switch strings.ToUpper(strings.TrimSpace(short)) {
	case "1H", "HT", "2H", "ET", "BT", "P", "SUSP", "INT", "LIVE":
		return StatusLive
	case "FT", "AET", "PEN", "AWD", "WO":
		return StatusFinished
	case "PST":
		return StatusPostponed
	case "CANC", "ABD":
		return StatusCancelled
	default: // NS, TBD and anything new
		return StatusScheduled
	}
}
