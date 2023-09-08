// Copyright 2022 Harness Inc. All rights reserved.
// Use of this source code is governed by the Polyform Free Trial License
// that can be found in the LICENSE.md file for this repository.

package trigger

import (
	"context"

	"github.com/harness/gitness/events"
	gitevents "github.com/harness/gitness/internal/events/git"
)

func (s *Service) handleEventBranchCreated(ctx context.Context,
	event *events.Event[*gitevents.BranchCreatedPayload]) error {
	return events.NewDiscardEventErrorf("not implemented")
}

func (s *Service) handleEventBranchUpdated(ctx context.Context,
	event *events.Event[*gitevents.BranchUpdatedPayload]) error {
	return events.NewDiscardEventErrorf("not implemented")
}