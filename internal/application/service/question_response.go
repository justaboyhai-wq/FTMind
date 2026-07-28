package service

import "strings"

func parseGeneratedQuestionLines(content string, questionCount int) []string {
	lines := strings.Split(content, "\n")
	questions := make([]string, 0, questionCount)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimLeft(line, "0123456789.-*) ")
		line = strings.TrimSpace(line)
		// The prompt allows low-value fragments (for example isolated
		// calendar/table rows) to opt out explicitly. Never persist or index
		// that control token as a user-facing generated question.
		if strings.EqualFold(line, "SKIP") {
			continue
		}
		if line != "" && len(line) > 5 {
			questions = append(questions, line)
			if len(questions) >= questionCount {
				break
			}
		}
	}

	return questions
}
