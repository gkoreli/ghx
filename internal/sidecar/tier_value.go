package sidecar

// Tier is the canonical escalation tier ID used in reports and telemetry.
type Tier string

const (
	// Tier0 means the answer came from session memory alone.
	Tier0 Tier = "tier0"
	// Tier1 means remote ghx evidence contributed to the answer.
	Tier1 Tier = "tier1"
	// Tier2 means local structural evidence contributed to the answer.
	Tier2 Tier = "tier2"
	// Tier3 is reserved for future deeper escalation.
	Tier3 Tier = "tier3"
)

var validTiers = map[Tier]bool{
	Tier0: true,
	Tier1: true,
	Tier2: true,
	Tier3: true,
}

// ParseTier validates a serialized tier ID.
func ParseTier(tier string) (Tier, bool) {
	t := Tier(tier)
	return t, validTiers[t]
}

// String returns the serialized tier ID.
func (t Tier) String() string {
	return string(t)
}
