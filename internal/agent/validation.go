// validation.go: sanitize input truoc khi dua vao prompt, chong prompt injection.
package agent

import "regexp"

var sanitizePattern = regexp.MustCompile(`[^\w\s.,\-]`)

func SanitizeField(value string, maxLen int) string {
	clean := sanitizePattern.ReplaceAllString(value, "")
	if len(clean) > maxLen {
		return clean[:maxLen]
	}
	return clean
}
