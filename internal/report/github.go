package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/get-felipe/pg-contract/internal/check"
)

func WriteGitHub(w io.Writer, report *check.Report) {
	breaking := report.Breaking()
	invalidBefore := report.InvalidBefore()

	for _, result := range invalidBefore {
		writeGitHubAnnotation(w, "error", result, "invalid before", result.Before.Error)
	}
	for _, result := range breaking {
		writeGitHubBreakingAnnotation(w, result)
	}

	if len(breaking) == 0 && len(invalidBefore) == 0 {
		message := fmt.Sprintf("%d query checked. No valid-before/fail-after breakages found.", len(report.Results))
		writeGitHubCommand(w, "notice", nil, message)
	}
}

func writeGitHubBreakingAnnotation(w io.Writer, result check.Result) {
	if result.ShapeChange != nil && result.After.OK {
		message := result.BreakingReason()
		if summary := result.BreakingSummary(); summary != "" {
			message += " " + summary + "."
		}
		writeGitHubAnnotationMessage(w, "error", result, "breaking", message)
		return
	}
	writeGitHubAnnotation(w, "error", result, "breaking", result.BreakingError())
}

func writeGitHubAnnotation(w io.Writer, level string, result check.Result, status string, err *check.DBError) {
	message := check.Reason(err)
	if err != nil {
		if err.Code != "" {
			message += " SQLSTATE: " + err.Code + "."
		}
		if err.Message != "" {
			message += " " + err.Message
		}
	}
	writeGitHubAnnotationMessage(w, level, result, status, message)
}

func writeGitHubAnnotationMessage(w io.Writer, level string, result check.Result, status string, message string) {
	title := fmt.Sprintf("pg-contract: %s: %s", status, result.Query.Name)
	properties := map[string]string{
		"file":  result.Query.File,
		"line":  fmt.Sprintf("%d", result.Query.StartLine),
		"title": title,
	}
	writeGitHubCommand(w, level, properties, message)
}

func writeGitHubCommand(w io.Writer, command string, properties map[string]string, message string) {
	fmt.Fprintf(w, "::%s", command)
	if len(properties) > 0 {
		ordered := []string{"file", "line", "col", "endLine", "endColumn", "title"}
		first := true
		for _, key := range ordered {
			value, ok := properties[key]
			if !ok || value == "" {
				continue
			}
			if first {
				fmt.Fprint(w, " ")
				first = false
			} else {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, "%s=%s", key, escapeGitHubProperty(value))
		}
	}
	fmt.Fprintf(w, "::%s\n", escapeGitHubData(message))
}

func escapeGitHubData(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	value = strings.ReplaceAll(value, "\r", "%0D")
	value = strings.ReplaceAll(value, "\n", "%0A")
	return value
}

func escapeGitHubProperty(value string) string {
	value = escapeGitHubData(value)
	value = strings.ReplaceAll(value, ":", "%3A")
	value = strings.ReplaceAll(value, ",", "%2C")
	return value
}
