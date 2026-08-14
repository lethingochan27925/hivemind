// validation.go: sanitize input truoc khi dua vao prompt, chong prompt injection.
package agent

import "regexp"

var sanitizePattern = regexp.MustCompile(`[^\w\s.,\-]`)

func SanitizeField(value string, maxLen int) string {
	clean := sanitizePattern.ReplaceAllString(value, "")
	// Bien do dai khong hop le -> khong cat, thay vi slice bang chi so am
	// (se panic). Mot ham lam sach dau vao khong duoc phep tu no lam sap worker.
	if maxLen <= 0 {
		return clean
	}
	if len(clean) > maxLen {
		return clean[:maxLen]
	}
	return clean
}
