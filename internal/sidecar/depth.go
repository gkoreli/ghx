package sidecar

// Depth is the canonical recon budget dial accepted by the sidecar.
type Depth string

const (
	// DepthCheap is the smallest recon budget.
	DepthCheap Depth = "cheap"
	// DepthNormal is the default recon budget.
	DepthNormal Depth = "normal"
	// DepthDeep is the largest recon budget.
	DepthDeep Depth = "deep"
)

var validDepths = map[Depth]bool{
	DepthCheap:  true,
	DepthNormal: true,
	DepthDeep:   true,
}

// ParseDepth validates a recon budget dial without changing caller policy.
// Callers decide whether an invalid value is rejected or coerced.
func ParseDepth(depth string) (Depth, bool) {
	d := Depth(depth)
	return d, validDepths[d]
}

// String returns the serialized depth value.
func (d Depth) String() string {
	return string(d)
}
