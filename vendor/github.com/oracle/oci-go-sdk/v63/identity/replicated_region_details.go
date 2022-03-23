// Copyright (c) 2016, 2018, 2022, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.
// Code generated. DO NOT EDIT.

// Identity and Access Management Service API
//
// APIs for managing users, groups, compartments, and policies.
//

package identity

import (
	"fmt"
	"github.com/oracle/oci-go-sdk/v63/common"
	"strings"
)

// ReplicatedRegionDetails Properties for a region where a domain is replicated too.
type ReplicatedRegionDetails struct {

	// A REPLICATION_ENABLED region, e.g. us-ashburn-1.
	// See Regions and Availability Domains (https://docs.cloud.oracle.com/Content/General/Concepts/regions.htm)
	// for the full list of supported region names.
	Region *string `mandatory:"false" json:"region"`

	// Region agnostic domain URL.
	Url *string `mandatory:"false" json:"url"`

	// The IDCS replicated region state
	State ReplicatedRegionDetailsStateEnum `mandatory:"false" json:"state,omitempty"`
}

func (m ReplicatedRegionDetails) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m ReplicatedRegionDetails) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if _, ok := GetMappingReplicatedRegionDetailsStateEnum(string(m.State)); !ok && m.State != "" {
		errMessage = append(errMessage, fmt.Sprintf("unsupported enum value for State: %s. Supported values are: %s.", m.State, strings.Join(GetReplicatedRegionDetailsStateEnumStringValues(), ",")))
	}
	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}

// ReplicatedRegionDetailsStateEnum Enum with underlying type: string
type ReplicatedRegionDetailsStateEnum string

// Set of constants representing the allowable values for ReplicatedRegionDetailsStateEnum
const (
	ReplicatedRegionDetailsStateEnablingReplication  ReplicatedRegionDetailsStateEnum = "ENABLING_REPLICATION"
	ReplicatedRegionDetailsStateReplicationEnabled   ReplicatedRegionDetailsStateEnum = "REPLICATION_ENABLED"
	ReplicatedRegionDetailsStateDisablingReplication ReplicatedRegionDetailsStateEnum = "DISABLING_REPLICATION"
	ReplicatedRegionDetailsStateReplicationDisabled  ReplicatedRegionDetailsStateEnum = "REPLICATION_DISABLED"
	ReplicatedRegionDetailsStateDeleted              ReplicatedRegionDetailsStateEnum = "DELETED"
)

var mappingReplicatedRegionDetailsStateEnum = map[string]ReplicatedRegionDetailsStateEnum{
	"ENABLING_REPLICATION":  ReplicatedRegionDetailsStateEnablingReplication,
	"REPLICATION_ENABLED":   ReplicatedRegionDetailsStateReplicationEnabled,
	"DISABLING_REPLICATION": ReplicatedRegionDetailsStateDisablingReplication,
	"REPLICATION_DISABLED":  ReplicatedRegionDetailsStateReplicationDisabled,
	"DELETED":               ReplicatedRegionDetailsStateDeleted,
}

var mappingReplicatedRegionDetailsStateEnumLowerCase = map[string]ReplicatedRegionDetailsStateEnum{
	"enabling_replication":  ReplicatedRegionDetailsStateEnablingReplication,
	"replication_enabled":   ReplicatedRegionDetailsStateReplicationEnabled,
	"disabling_replication": ReplicatedRegionDetailsStateDisablingReplication,
	"replication_disabled":  ReplicatedRegionDetailsStateReplicationDisabled,
	"deleted":               ReplicatedRegionDetailsStateDeleted,
}

// GetReplicatedRegionDetailsStateEnumValues Enumerates the set of values for ReplicatedRegionDetailsStateEnum
func GetReplicatedRegionDetailsStateEnumValues() []ReplicatedRegionDetailsStateEnum {
	values := make([]ReplicatedRegionDetailsStateEnum, 0)
	for _, v := range mappingReplicatedRegionDetailsStateEnum {
		values = append(values, v)
	}
	return values
}

// GetReplicatedRegionDetailsStateEnumStringValues Enumerates the set of values in String for ReplicatedRegionDetailsStateEnum
func GetReplicatedRegionDetailsStateEnumStringValues() []string {
	return []string{
		"ENABLING_REPLICATION",
		"REPLICATION_ENABLED",
		"DISABLING_REPLICATION",
		"REPLICATION_DISABLED",
		"DELETED",
	}
}

// GetMappingReplicatedRegionDetailsStateEnum performs case Insensitive comparison on enum value and return the desired enum
func GetMappingReplicatedRegionDetailsStateEnum(val string) (ReplicatedRegionDetailsStateEnum, bool) {
	enum, ok := mappingReplicatedRegionDetailsStateEnumLowerCase[strings.ToLower(val)]
	return enum, ok
}
