// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package apiv2

import (
	"context"
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
)

type taskEventListBody struct {
	Body Paginated[*models.TaskEvent]
}

func RegisterTaskEventRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID: "task-events-list",
		Summary:     "List a task's change history",
		Description: "Returns who changed the assignees or the end date of a task and when, oldest first. The whole history is returned in one page. Visible to everyone who can read the task.",
		Method:      http.MethodGet,
		Path:        "/tasks/{task}/events",
		Tags:        []string{"tasks"},
	}, taskEventsList)
}

func init() { AddRouteRegistrar(RegisterTaskEventRoutes) }

func taskEventsList(ctx context.Context, in *struct {
	TaskID int64 `path:"task"`
	ListParams
}) (*taskEventListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.TaskEvent{TaskID: in.TaskID}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.TaskEvent)
	if !ok {
		return nil, fmt.Errorf("taskEvents.ReadAll returned unexpected type %T (expected []*models.TaskEvent)", result)
	}
	return &taskEventListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}
