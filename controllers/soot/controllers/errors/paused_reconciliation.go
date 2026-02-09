// Copyright 2026 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"github.com/pkg/errors"
)

var ErrPausedReconciliation = errors.New("paused reconciliation, no further actions")
