package agents

import "github.com/gentleman-programming/gentle-ai/internal/model"

// RuntimeCapabilitiesProvider is an optional interface that adapters can
// implement to declare their runtime capabilities explicitly.
//
// Adapters that don't implement this interface will have their capabilities
// derived from DelegationModel() for backward compatibility.
type RuntimeCapabilitiesProvider interface {
	// RuntimeCapabilities returns the capabilities this runtime provides.
	RuntimeCapabilities() []model.RuntimeCapability
}

// GetRuntimeCapabilities extracts runtime capabilities from an adapter.
//
// If the adapter implements RuntimeCapabilitiesProvider, those capabilities
// are returned directly. Otherwise, capabilities are derived from the
// adapter's DelegationModel() for backward compatibility.
//
// This design allows gradual migration: existing adapters work unchanged,
// while new adapters can opt into explicit capability declaration.
func GetRuntimeCapabilities(adapter Adapter) []model.RuntimeCapability {
	// Check if adapter explicitly provides capabilities
	if provider, ok := adapter.(RuntimeCapabilitiesProvider); ok {
		return provider.RuntimeCapabilities()
	}

	// Fall back to deriving from DelegationModel
	return model.RuntimeCapabilitiesFromDelegationModel(adapter.DelegationModel())
}

// GetRuntimeCapabilitySet is a convenience function that returns capabilities
// as a set for efficient lookups during resolution.
func GetRuntimeCapabilitySet(adapter Adapter) model.RuntimeCapabilitySet {
	return model.NewRuntimeCapabilitySet(GetRuntimeCapabilities(adapter))
}
