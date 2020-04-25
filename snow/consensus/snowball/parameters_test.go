// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowball

import (
	"fmt"
	"strings"
	"testing"
)

func TestParametersValid(t *testing.T) {
	p := Parameters{
		K:                 1,
		Alpha:             1,
		BetaVirtuous:      1,
		BetaRogue:         1,
		ConcurrentRepolls: 1,
	}

	if err := p.Valid(); err != nil {
		t.Fatal(err)
	}
}

func TestParametersAnotherValid(t *testing.T) {
	p := Parameters{
		K:                 1,
		Alpha:             1,
		BetaVirtuous:      28,
		BetaRogue:         30,
		ConcurrentRepolls: 1,
	}

	if err := p.Valid(); err != nil {
		t.Fatal(err)
	}
}

func TestParametersYetAnotherValid(t *testing.T) {
	p := Parameters{
		K:                 1,
		Alpha:             1,
		BetaVirtuous:      3,
		BetaRogue:         3,
		ConcurrentRepolls: 1,
	}

	if err := p.Valid(); err != nil {
		t.Fatal(err)
	}
}

func TestParametersInvalidK(t *testing.T) {
	p := Parameters{
		K:                 0,
		Alpha:             1,
		BetaVirtuous:      1,
		BetaRogue:         1,
		ConcurrentRepolls: 1,
	}

	if err := p.Valid(); err == nil {
		t.Fatalf("Should have failed due to invalid k")
	}
}

func TestParametersInvalidAlpha(t *testing.T) {
	p := Parameters{
		K:                 1,
		Alpha:             0,
		BetaVirtuous:      1,
		BetaRogue:         1,
		ConcurrentRepolls: 1,
	}

	if err := p.Valid(); err == nil {
		t.Fatalf("Should have failed due to invalid alpha")
	}
}

func TestParametersInvalidBetaVirtuous(t *testing.T) {
	p := Parameters{
		K:                 1,
		Alpha:             1,
		BetaVirtuous:      0,
		BetaRogue:         1,
		ConcurrentRepolls: 1,
	}

	if err := p.Valid(); err == nil {
		t.Fatalf("Should have failed due to invalid beta virtuous")
	}
}

func TestParametersInvalidBetaRogue(t *testing.T) {
	p := Parameters{
		K:                 1,
		Alpha:             1,
		BetaVirtuous:      1,
		BetaRogue:         0,
		ConcurrentRepolls: 1,
	}

	if err := p.Valid(); err == nil {
		t.Fatalf("Should have failed due to invalid beta rogue")
	}
}

func TestParametersAnotherInvalidBetaRogue(t *testing.T) {
	p := Parameters{
		K:                 1,
		Alpha:             1,
		BetaVirtuous:      28,
		BetaRogue:         3,
		ConcurrentRepolls: 1,
	}

	if err := p.Valid(); err == nil {
		t.Fatalf("Should have failed due to invalid beta rogue")
	} else if !strings.Contains(err.Error(), "\n") {
		t.Fatalf("Should have described the extensive error")
	}
}

func TestParametersInvalidConcurrentRepolls(t *testing.T) {
	tests := []Parameters{
		{
			K:                 1,
			Alpha:             1,
			BetaVirtuous:      1,
			BetaRogue:         1,
			ConcurrentRepolls: 2,
		},
		{
			K:                 1,
			Alpha:             1,
			BetaVirtuous:      1,
			BetaRogue:         1,
			ConcurrentRepolls: 0,
		},
	}
	for _, p := range tests {
		label := fmt.Sprintf("ConcurrentRepolls=%d", p.ConcurrentRepolls)
		t.Run(label, func(t *testing.T) {
			if err := p.Valid(); err == nil {
				t.Error("Should have failed due to invalid concurrent repolls")
			}
		})
	}
}
