package lower

import (
	"fmt"
	"sort"

	"github.com/jacoelho/rq/internal/pm/diagnostics"
	"github.com/jacoelho/rq/internal/pm/report"
)

func buildUnmappedIssues(counts map[report.IssueCode]int, firstLine map[report.IssueCode]int, total int) []report.Issue {
	if total == 0 {
		return nil
	}

	codes := make([]string, 0, len(counts))
	for code := range counts {
		codes = append(codes, string(code))
	}
	sort.Strings(codes)

	issues := make([]report.Issue, 0, len(codes)+1)
	for _, code := range codes {
		issueCode := report.IssueCode(code)
		issue := report.Issue{
			Code:     issueCode,
			Stage:    diagnostics.StageLower,
			Severity: diagnostics.SeverityError,
			Message:  fmt.Sprintf("%d script lines were not mapped (%s)", counts[issueCode], issueCode),
		}
		if line := firstLine[issueCode]; line > 0 {
			issue.Span = &diagnostics.Span{Line: line}
		}
		issues = append(issues, issue)
	}

	issues = append(issues, report.Issue{
		Code:     report.CodeTestNotMapped,
		Stage:    diagnostics.StageLower,
		Severity: diagnostics.SeverityError,
		Message:  fmt.Sprintf("%d test script lines were not mapped", total),
	})

	return issues
}
