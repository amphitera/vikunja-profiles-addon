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
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
)

func RegisterTeamMemberRoleRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID: "teams-members-set-role",
		Summary:     "Set a team member's role",
		Description: "Replaces the free-text role of a team member (e.g. \"designer\"). Send an empty role to clear it. Only a team admin may do this. The role is returned with the team's members.",
		Method:      http.MethodPut,
		Path:        "/teams/{team}/members/{user}/role",
		Tags:        []string{"teams"},
	}, teamMembersSetRole)
}

func init() { AddRouteRegistrar(RegisterTeamMemberRoleRoutes) }

func teamMembersSetRole(ctx context.Context, in *struct {
	TeamID   int64  `path:"team"`
	Username string `path:"user" doc:"The username of the member whose role to set."`
	Body     models.TeamMemberRole
}) (*singleBody[models.TeamMemberRole], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	in.Body.TeamID = in.TeamID
	in.Body.Username = in.Username
	if err := handler.DoUpdate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.TeamMemberRole]{Body: &in.Body}, nil
}
