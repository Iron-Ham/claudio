package plan

// IssueCreationResult holds the results of creating all issues
type IssueCreationResult struct {
	ParentIssueNumber int
	ParentIssueURL    string
	SubIssueNumbers   map[string]int    // task_id -> issue_number
	SubIssueURLs      map[string]string // task_id -> issue_url
	GroupIssueNumbers []int             // group issue numbers (for groups with >1 task)
	GroupIssueURLs    []string          // group issue URLs
}
