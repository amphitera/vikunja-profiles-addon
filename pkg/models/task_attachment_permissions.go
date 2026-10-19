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
	"code.vikunja.io/api/pkg/web"
	"xorm.io/xorm"
)

// CanRead checks if the user can see an attachment
func (ta *TaskAttachment) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	t := &Task{ID: ta.TaskID}
	return t.CanRead(s, a)
}

// CanDelete checks if the user can delete an attachment
func (ta *TaskAttachment) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	t := &Task{ID: ta.TaskID}
	canWrite, err := t.CanWrite(s, a)
	if err != nil || canWrite {
		return canWrite, err
	}

	// Assignees without write access may only remove what they uploaded themselves.
	isAssignee, err := canActAsAssignee(s, a, ta.TaskID)
	if err != nil || !isAssignee {
		return false, err
	}
	return s.
		Where("id = ? AND task_id = ? AND created_by_id = ?", ta.ID, ta.TaskID, a.GetID()).
		Exist(&TaskAttachment{})
}

// CanCreate checks if the user can create an attachment
func (ta *TaskAttachment) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	t, err := GetTaskByIDSimple(s, ta.TaskID)
	if err != nil {
		return false, err
	}
	canCreate, err := t.CanCreate(s, a)
	if err != nil || canCreate {
		return canCreate, err
	}
	return canActAsAssignee(s, a, ta.TaskID)
}
