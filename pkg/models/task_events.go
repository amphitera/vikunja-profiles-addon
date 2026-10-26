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
	"time"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

const (
	TaskEventAssigneeAdded   = "assignee_added"
	TaskEventAssigneeRemoved = "assignee_removed"
	TaskEventEndDateChanged  = "end_date_changed"
)

// TaskEvent is one line of a task's change history.
type TaskEvent struct {
	ID       int64      `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The unique, numeric id of this event."`
	TaskID   int64      `xorm:"bigint not null INDEX" json:"task_id" readOnly:"true" doc:"The task this event belongs to."`
	UserID   int64      `xorm:"bigint not null" json:"-"`
	User     *user.User `xorm:"-" json:"user" readOnly:"true" doc:"Who made the change. Null if the user no longer exists."`
	Kind     string     `xorm:"varchar(50) not null" json:"kind" readOnly:"true" enum:"assignee_added,assignee_removed,end_date_changed" doc:"What changed."`
	OldValue string     `xorm:"text null" json:"old_value" readOnly:"true" doc:"Value before the change: a display name for assignee events, an RFC 3339 date or empty for date events."`
	NewValue string     `xorm:"text null" json:"new_value" readOnly:"true" doc:"Value after the change, same format as old_value."`
	Created  time.Time  `xorm:"created not null" json:"created" readOnly:"true" doc:"When the change was made."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

func (*TaskEvent) TableName() string {
	return "task_events"
}

func addTaskEvent(s *xorm.Session, taskID int64, a web.Auth, kind, oldValue, newValue string) error {
	var doerID int64
	if a != nil {
		doerID = a.GetID()
	}
	_, err := s.Insert(&TaskEvent{
		TaskID:   taskID,
		UserID:   doerID,
		Kind:     kind,
		OldValue: oldValue,
		NewValue: newValue,
	})
	return err
}

func formatEventDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// CanRead allows everyone who can read the task to see its history.
func (e *TaskEvent) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	return (&Task{ID: e.TaskID}).CanRead(s, a)
}

// ReadAll returns the history of a task, oldest first.
func (e *TaskEvent) ReadAll(s *xorm.Session, a web.Auth, _ string, _ int, _ int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	can, _, err := e.CanRead(s, a)
	if err != nil {
		return nil, 0, 0, err
	}
	if !can {
		return nil, 0, 0, ErrGenericForbidden{}
	}

	taskEvents := []*TaskEvent{}
	err = s.Where(builder.Eq{"task_id": e.TaskID}).OrderBy("created ASC, id ASC").Find(&taskEvents)
	if err != nil {
		return nil, 0, 0, err
	}

	userIDs := make([]int64, 0, len(taskEvents))
	for _, ev := range taskEvents {
		userIDs = append(userIDs, ev.UserID)
	}
	users, err := user.GetUsersByIDs(s, userIDs)
	if err != nil {
		return nil, 0, 0, err
	}
	for _, ev := range taskEvents {
		ev.User = users[ev.UserID]
	}

	return taskEvents, len(taskEvents), int64(len(taskEvents)), nil
}
