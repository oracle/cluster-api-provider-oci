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

// DomainReplication Domain replication states.
type DomainReplication struct {

	// Version number indicating the value of kievTxnId, starting from which, the domain replication events need to be returned.
	OpcWaterMark *float32 `mandatory:"true" json:"opcWaterMark"`

	// Custom value defining the order of records with same kievTxnId
	TxnSeqNumber *float32 `mandatory:"true" json:"txnSeqNumber"`

	// The domain's replication state
	DomainReplicationStates []DomainReplicationStates `mandatory:"true" json:"domainReplicationStates"`
}

func (m DomainReplication) String() string {
	return common.PointerString(m)
}

// ValidateEnumValue returns an error when providing an unsupported enum value
// This function is being called during constructing API request process
// Not recommended for calling this function directly
func (m DomainReplication) ValidateEnumValue() (bool, error) {
	errMessage := []string{}

	if len(errMessage) > 0 {
		return true, fmt.Errorf(strings.Join(errMessage, "\n"))
	}
	return false, nil
}
