package issueopsnext

import (
	"fmt"
	"time"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsinventorydomain "issueops/internal/domain/issueopsinventory"
)

// idleWarningDays is the number of whole idle days at which next warns that a
// cycle has stalled. It is a judgment value with no measured distribution
// behind it; collect the idle_days distribution of real cycles and set it again.
const idleWarningDays = 7

// IdleWarning returns the stall warning for record at now, or "" when the cycle
// has written a record within idleWarningDays or its last record time is
// unknown or in the future. It reuses the inventory LastRecordAt and IdleDays
// so next and list agree on what idle means.
func IdleWarning(record issueopsnextcontract.Record, now time.Time) string {
	last := issueopsinventorydomain.LastRecordAt(record)
	days := issueopsinventorydomain.IdleDays(last, now)
	if days == nil || *days < idleWarningDays {
		return ""
	}
	return fmt.Sprintf("cycle %s has had no record write for %d days (last_record_at %s); ask the user whether it is still in progress", record.ID, *days, last)
}
