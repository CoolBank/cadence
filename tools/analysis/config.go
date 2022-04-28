/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package analysis

import (
	"fmt"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
)

// A Config specifies details about how programs should be loaded.
// The zero value is a valid configuration.
// Calls to Load do not modify this struct.
type Config struct {
	// Mode controls the level of information returned for each program.
	Mode LoadMode

	// ResolveAddressContractNames is called to resolve the contract names of an address location.
	ResolveAddressContractNames func(address common.Address) ([]string, error)

	// ResolveCode is called to resolve an import to its source code.
	ResolveCode func(
		location common.Location,
		importingLocation common.Location,
		importRange ast.Range,
	) (string, error)
}

func NewSimpleConfig(
	mode LoadMode,
	codes map[common.LocationID]string,
	contractNames map[common.Address][]string,
	resolveAddressContracts func(common.Address) (contracts map[string]string, err error),
) *Config {

	loadAddressContracts := func(address common.Address) error {
		if resolveAddressContracts == nil {
			return nil
		}
		contracts, err := resolveAddressContracts(address)
		if err != nil {
			return err
		}

		names := make([]string, 0, len(contracts))

		for name, code := range contracts {
			location := common.AddressLocation{
				Address: address,
				Name:    name,
			}
			codes[location.ID()] = code
			names = append(names, name)
		}

		contractNames[address] = names

		return nil
	}

	config := &Config{
		Mode: mode,
		ResolveAddressContractNames: func(
			address common.Address,
		) (
			[]string,
			error,
		) {
			repeat := true
			for {
				names, ok := contractNames[address]
				if !ok {
					if repeat {
						err := loadAddressContracts(address)
						if err != nil {
							return nil, err
						}
						repeat = false
						continue
					}

					return nil, fmt.Errorf(
						"missing contracts for address: %s",
						address,
					)
				}
				return names, nil
			}
		},
		ResolveCode: func(
			location common.Location,
			importingLocation common.Location,
			importRange ast.Range,
		) (
			string,
			error,
		) {
			repeat := true
			for {
				code, ok := codes[location.ID()]
				if !ok {
					if repeat {
						if addressLocation, ok := location.(common.AddressLocation); ok {
							err := loadAddressContracts(addressLocation.Address)
							if err != nil {
								return "", err
							}
							repeat = false
							continue
						}
					}

					return "", fmt.Errorf(
						"import of unknown location: %s",
						location,
					)
				}

				return code, nil
			}
		},
	}
	return config
}
