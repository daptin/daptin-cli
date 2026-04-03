package cmd

import (
	"fmt"
	"strings"
)

var operationNames = []string{"Peek", "Read", "Create", "Update", "Delete", "Execute", "Refer"}
var tierNames = []string{"Guest", "Owner", "Group"}
var tierShifts = []int{0, 7, 14}

// Permission represents decoded permission bits.
type Permission struct {
	Guest []string
	Owner []string
	Group []string
}

// DecodePermission breaks a 21-bit permission value into named operations per tier.
// Pure function.
func DecodePermission(value int64) Permission {
	return Permission{
		Guest: decodeTier(value, 0),
		Owner: decodeTier(value, 7),
		Group: decodeTier(value, 14),
	}
}

func decodeTier(value int64, shift int) []string {
	bits := (value >> shift) & 0x7F
	var ops []string
	for i, name := range operationNames {
		if bits&(1<<i) != 0 {
			ops = append(ops, name)
		}
	}
	return ops
}

// EncodePermission encodes named operations back to a 21-bit value.
// Pure function.
func EncodePermission(p Permission) int64 {
	return encodeTier(p.Guest, 0) | encodeTier(p.Owner, 7) | encodeTier(p.Group, 14)
}

func encodeTier(ops []string, shift int) int64 {
	var bits int64
	for _, op := range ops {
		for i, name := range operationNames {
			if op == name {
				bits |= 1 << i
			}
		}
	}
	return bits << shift
}

// FormatPermission returns a human-readable multi-line string.
// Pure function.
func FormatPermission(value int64) string {
	p := DecodePermission(value)
	var sb strings.Builder

	for i, tier := range [][]string{p.Guest, p.Owner, p.Group} {
		sb.WriteString(fmt.Sprintf("  %s: ", tierNames[i]))
		if len(tier) == 0 {
			sb.WriteString("(none)")
		} else {
			sb.WriteString(strings.Join(tier, ", "))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ApplyPermissionModifier applies a +TierOp or -TierOp modifier to a permission value.
// E.g., "+GuestRead" adds Guest Read, "-OwnerDelete" removes Owner Delete.
// Pure function.
func ApplyPermissionModifier(value int64, modifier string) (int64, error) {
	if len(modifier) < 2 {
		return 0, fmt.Errorf("invalid permission modifier %q", modifier)
	}

	action := modifier[0]
	if action != '+' && action != '-' {
		return 0, fmt.Errorf("permission modifier must start with + or -, got %q", modifier)
	}

	name := modifier[1:]

	tierIdx := -1
	opIdx := -1
	for ti, tn := range tierNames {
		for oi, on := range operationNames {
			if name == tn+on {
				tierIdx = ti
				opIdx = oi
			}
		}
	}

	if tierIdx < 0 || opIdx < 0 {
		return 0, fmt.Errorf("unknown permission %q (expected format: Guest|Owner|Group + Peek|Read|Create|Update|Delete|Execute|Refer)", name)
	}

	bit := int64(1<<opIdx) << tierShifts[tierIdx]
	if action == '+' {
		return value | bit, nil
	}
	return value &^ bit, nil
}
