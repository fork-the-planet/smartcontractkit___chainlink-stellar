package operations

import "github.com/Masterminds/semver/v3"

// ContractDeploymentVersion is the semver stamped on Stellar Soroban deployment
// operations in this tree until contract releases are versioned independently.
var ContractDeploymentVersion = semver.MustParse("2.0.0")
