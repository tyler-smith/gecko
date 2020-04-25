// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package avm

import (
	"testing"
)

func TestBuildGenesis(t *testing.T) {
	ss := StaticService{}

	args := BuildGenesisArgs{GenesisData: map[string]AssetDefinition{
		"asset1": {
			Name:         "myFixedCapAsset",
			Symbol:       "MFCA",
			Denomination: 8,
			InitialState: map[string][]interface{}{
				"fixedCap": {
					Holder{
						Amount:  100000,
						Address: "A9bTQjfYGBFK3JPRJqF2eh3JYL7cHocvy",
					},
					Holder{
						Amount:  100000,
						Address: "6mxBGnjGDCKgkVe7yfrmvMA7xE7qCv3vv",
					},
					Holder{
						Amount:  50000,
						Address: "6ncQ19Q2U4MamkCYzshhD8XFjfwAWFzTa",
					},
					Holder{
						Amount:  50000,
						Address: "Jz9ayEDt7dx9hDx45aXALujWmL9ZUuqe7",
					},
				},
			},
		},
		"asset2": {
			Name:   "myVarCapAsset",
			Symbol: "MVCA",
			InitialState: map[string][]interface{}{
				"variableCap": {
					Owners{
						Threshold: 1,
						Minters: []string{
							"A9bTQjfYGBFK3JPRJqF2eh3JYL7cHocvy",
							"6mxBGnjGDCKgkVe7yfrmvMA7xE7qCv3vv",
						},
					},
					Owners{
						Threshold: 2,
						Minters: []string{
							"6ncQ19Q2U4MamkCYzshhD8XFjfwAWFzTa",
							"Jz9ayEDt7dx9hDx45aXALujWmL9ZUuqe7",
						},
					},
				},
			},
		},
		"asset3": {
			Name: "myOtherVarCapAsset",
			InitialState: map[string][]interface{}{
				"variableCap": {
					Owners{
						Threshold: 1,
						Minters: []string{
							"A9bTQjfYGBFK3JPRJqF2eh3JYL7cHocvy",
						},
					},
				},
			},
		},
	}}
	reply := BuildGenesisReply{}
	err := ss.BuildGenesis(nil, &args, &reply)
	if err != nil {
		t.Fatal(err)
	}
}
