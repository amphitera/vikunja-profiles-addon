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

package models

import (
	"fmt"
	"net/http"
	"strings"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// TeamMemberRole sets the free-text role of a team member. It is a separate
// type rather than a TeamMember field update because TeamMember.Update is the
// admin toggle and v1 clients depend on that.
type TeamMemberRole struct {
	TeamID   int64  `xorm:"-" json:"-"`
	Username string `xorm:"-" json:"username" readOnly:"true" doc:"The username of the member. Taken from the URL."`
	Role     string `xorm:"-" json:"role" maxLength:"250" doc:"The member's role in the team, free text (e.g. \"designer\"). An empty string clears it."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// CanUpdate allows team admins (and instance admins) to change roles.
func (r *TeamMemberRole) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	return (&TeamMember{TeamID: r.TeamID}).IsAdmin(s, a)
}

// Update stores the role of the member.
func (r *TeamMemberRole) Update(s *xorm.Session, _ web.Auth) error {
	u, err := user.GetUserByUsername(s, r.Username)
	if err != nil && !user.IsErrUserStatusError(err) {
		return err
	}

	r.Role = strings.TrimSpace(r.Role)
	if len([]rune(r.Role)) > 250 {
		return ErrTeamMemberRoleTooLong{}
	}

	exists, err := (&TeamMember{TeamID: r.TeamID, UserID: u.ID}).MembershipExists(s)
	if err != nil {
		return err
	}
	if !exists {
		return ErrUserIsNotMemberOfTeam{TeamID: r.TeamID, UserID: u.ID}
	}

	_, err = s.
		Where("team_id = ? AND user_id = ?", r.TeamID, u.ID).
		Cols("profile_role").
		Update(&TeamMember{Role: r.Role})
	return err
}

// Codes in the 69xx range stay clear of upstream's team error codes (60xx).
const (
	ErrCodeUserIsNotMemberOfTeam = 6901
	ErrCodeTeamMemberRoleTooLong = 6902
)

// ErrUserIsNotMemberOfTeam represents an error where a user is not a member of a team.
type ErrUserIsNotMemberOfTeam struct {
	TeamID int64
	UserID int64
}

func (err ErrUserIsNotMemberOfTeam) Error() string {
	return fmt.Sprintf("User is not a member of that team. [Team ID: %d, User ID: %d]", err.TeamID, err.UserID)
}

// HTTPError holds the http error description
func (err ErrUserIsNotMemberOfTeam) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusNotFound, Code: ErrCodeUserIsNotMemberOfTeam, Message: "This user is not a member of that team."}
}

// ErrTeamMemberRoleTooLong represents an error where a member role exceeds 250 characters.
type ErrTeamMemberRoleTooLong struct{}

func (err ErrTeamMemberRoleTooLong) Error() string {
	return "Team member role is too long."
}

// HTTPError holds the http error description
func (err ErrTeamMemberRoleTooLong) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusBadRequest, Code: ErrCodeTeamMemberRoleTooLong, Message: "The role cannot be longer than 250 characters."}
}
