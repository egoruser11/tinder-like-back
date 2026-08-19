package models

// DiscoveryCandidate is the database representation of a profile considered
// for the discovery feed. It is intentionally not an HTTP response.
type DiscoveryCandidate struct {
	Profile    Profile
	IsVerified bool
}
