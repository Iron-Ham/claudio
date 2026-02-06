package mailbox

import "sort"

// sortMessages sorts messages chronologically by timestamp.
func sortMessages(msgs []Message) {
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].Timestamp.Before(msgs[j].Timestamp)
	})
}
