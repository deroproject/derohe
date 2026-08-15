// Copyright 2017-2025 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
//
// Security enhancement: Payload proof validation
// Prevents fake proofs with impossible amounts

package proof

import "fmt"

const (
	// Maximum safe value for int64 conversion (2^63 - 1)
	// Prevents uint64 to int64 wraparound attacks
	maxInt64Safe = 9223372036854775807

	// For display and calculations: 1 DERO = 100,000 atomic units
	// (matches globals/globals.go:295 and rpc_dero_getinfo.go:81)
	atomicUnitsPerDero = 100_000

	// DERO hard cap: 21M DERO (like Bitcoin, this will never increase)
	deroHardCapAtomic = 21_000_000 * atomicUnitsPerDero // 2_100_000_000_000

	// Setting max at 22M DERO = ~5% above hard cap.
	// Blocks impossible amounts while leaving a buffer.
	maxReasonableAmountAtomic = 22_000_000 * atomicUnitsPerDero // 2_200_000_000_000

	// Current approximate circulating supply (for context only,
	// used by DetectSuspiciousProofPatterns).
	currentSupplyApproxAtomic = 16_500_000 * atomicUnitsPerDero // 1_650_000_000_000
)

// ValidatePayloadProofAmount performs security checks on payload proof amounts
// Prevents:
//   - uint64 to int64 wraparound attacks
//   - Impossibly large amounts (exceeding total supply hard cap)
//   - False accusations based on fake proofs
func ValidatePayloadProofAmount(amount uint64) error {
	// Check 1: Prevent int64 wraparound
	// Historical context: Some code paths use SetInt64(int64(amount))
	// For amounts >= 2^63, this wraps to negative values
	if amount > maxInt64Safe {
		return fmt.Errorf("amount %d exceeds maximum safe integer (%d) - possible wraparound attack",
			amount, maxInt64Safe)
	}

	// Check 2: Sanity check against DERO hard cap
	// DERO has a permanent hard cap of 21M (like Bitcoin)
	// Any amount > 21M DERO is mathematically impossible
	// We use 22M to give a small buffer while blocking obvious fakes
	if amount > maxReasonableAmountAtomic {
		amountInDero := amount / atomicUnitsPerDero
		return fmt.Errorf("amount %d DERO exceeds DERO hard cap (21M) - proof is fabricated",
			amountInDero)
	}

	return nil
}

// DetectSuspiciousProofPatterns flags potentially fake or suspicious proofs
// Returns warnings for amounts that might warrant additional scrutiny
func DetectSuspiciousProofPatterns(amount uint64) []string {
	var warnings []string

	// Warning 1: Near int64 boundary (possible wraparound attempt)
	boundaryThreshold := uint64(maxInt64Safe * 9 / 10) // Within 90% of wraparound
	if amount > boundaryThreshold {
		warnings = append(warnings, "amount near int64 maximum - possible wraparound attempt")
	}

	// Warning 2: Exceeds current circulating supply (~16.5M)
	// This is suspicious but not impossible (could be legitimate edge case)
	if amount > currentSupplyApproxAtomic {
		amountDero := amount / atomicUnitsPerDero
		warnings = append(warnings,
			fmt.Sprintf("amount (%d DERO) exceeds current circulating supply - verify carefully", amountDero))
	}

	// Warning 3: Very large amount (> 1M DERO) - not fake, just notable
	largeThreshold := uint64(1_000_000 * atomicUnitsPerDero) // 1M DERO
	if amount > largeThreshold && amount <= currentSupplyApproxAtomic {
		amountDero := amount / atomicUnitsPerDero
		warnings = append(warnings, fmt.Sprintf("large transfer amount: %d DERO", amountDero))
	}

	// Warning 4: Suspiciously round number (exact multiple of 1M DERO) - often fabricated
	roundThreshold := uint64(1_000_000 * atomicUnitsPerDero) // 1M DERO
	if amount >= roundThreshold && amount%roundThreshold == 0 {
		amountDero := amount / atomicUnitsPerDero
		warnings = append(warnings, fmt.Sprintf("suspiciously round number (%d DERO exactly)", amountDero))
	}

	return warnings
}
