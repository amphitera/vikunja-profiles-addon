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

// canActAsAssignee grants read-only project members a few task actions —
// commenting, attaching files, moving their card between non-done buckets —
// on tasks assigned to them, without the full write access that would also
// let them close, edit or create tasks.
func canActAsAssignee(s *xorm.Session, a web.Auth, taskID int64) (bool, error) {
	if _, is := a.(*LinkSharing); is {
		return false, nil
	}

	canRead, _, err := (&Task{ID: taskID}).CanRead(s, a)
	if err != nil || !canRead {
		return false, err
	}

	return s.
		Where("task_id = ? AND user_id = ?", taskID, a.GetID()).
		Exist(&TaskAssginee{})
}

func canWriteOrActAsAssignee(s *xorm.Session, a web.Auth, taskID int64) (bool, error) {
	canWrite, err := (&Task{ID: taskID}).CanWrite(s, a)
	if err != nil || canWrite {
		return canWrite, err
	}
	return canActAsAssignee(s, a, taskID)
}

// canMoveAsAssignee lets an assignee move their task between buckets, but
// never into or out of the done bucket: that would change the done state,
// which stays reserved for members with write access.
func (b *TaskBucket) canMoveAsAssignee(s *xorm.Session, a web.Auth) (bool, error) {
	isAssignee, err := canActAsAssignee(s, a, b.TaskID)
	if err != nil || !isAssignee {
		return false, err
	}

	view, err := GetProjectViewByIDAndProject(s, b.ProjectViewID, b.ProjectID)
	if err != nil {
		return false, err
	}
	task, err := GetTaskByIDSimple(s, b.TaskID)
	if err != nil {
		return false, err
	}
	if task.ProjectID != view.ProjectID {
		return false, nil
	}
	bucket, err := getBucketByID(s, b.BucketID)
	if err != nil {
		return false, err
	}
	if bucket.ProjectViewID != view.ID {
		return false, nil
	}

	if view.DoneBucketID != 0 && (bucket.ID == view.DoneBucketID || task.Done) {
		return false, nil
	}
	return true, nil
}
