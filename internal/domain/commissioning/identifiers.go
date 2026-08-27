package commissioning

import (
	"fmt"
	"strings"
)

func NormalizeZoneCode(v string) string { return strings.ToUpper(strings.TrimSpace(v)) }
func NormalizeCategory(v string) string { return strings.TrimSpace(v) }
func PermitLabel(p ActivationPermit) string {
	return fmt.Sprintf("%s/%s/%s", p.PermitCode, p.CaseID, p.Status)
}
func IsTerminal(s State) bool { return s == Activated }
