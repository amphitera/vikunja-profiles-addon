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
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTask_DueDateFollowsEndDate(t *testing.T) {
	u := &user.User{ID: 1}
	end := time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)

	t.Run("setting the end date sets the due date and records it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		task := &Task{ID: 1, Title: "task #1", ProjectID: 1, EndDate: end}
		require.NoError(t, task.Update(s, u))
		require.NoError(t, s.Commit())

		assert.True(t, task.DueDate.Equal(end))
		db.AssertExists(t, "task_events", map[string]interface{}{
			"task_id": 1,
			"user_id": 1,
			"kind":    TaskEventEndDateChanged,
		}, false)
	})

	t.Run("updating only the end date column also writes the due date", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		task := &Task{ID: 1, ProjectID: 1, EndDate: end}
		require.NoError(t, task.updateSingleTask(s, u, []string{"end_date"}))
		require.NoError(t, s.Commit())

		s2 := db.NewSession()
		defer s2.Close()
		stored, err := GetTaskByIDSimple(s2, 1)
		require.NoError(t, err)
		assert.True(t, stored.DueDate.Equal(end))
	})

	t.Run("clearing the end date clears a due date that followed it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		task := &Task{ID: 1, Title: "task #1", ProjectID: 1, EndDate: end}
		require.NoError(t, task.Update(s, u))
		task = &Task{ID: 1, Title: "task #1", ProjectID: 1}
		require.NoError(t, task.Update(s, u))
		require.NoError(t, s.Commit())

		assert.True(t, task.DueDate.IsZero())
	})
}

func TestTaskEvent_ReadAll(t *testing.T) {
	t.Run("assignee changes are recorded and visible to readers", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		u := &user.User{ID: 1}
		assignee := &TaskAssginee{TaskID: 1, UserID: 1}
		require.NoError(t, assignee.Create(s, u))
		require.NoError(t, assignee.Delete(s, u))
		require.NoError(t, s.Commit())

		s2 := db.NewSession()
		defer s2.Close()
		result, _, _, err := (&TaskEvent{TaskID: 1}).ReadAll(s2, u, "", 1, 50)
		require.NoError(t, err)
		events := result.([]*TaskEvent)
		require.Len(t, events, 2)
		assert.Equal(t, TaskEventAssigneeAdded, events[0].Kind)
		assert.Equal(t, TaskEventAssigneeRemoved, events[1].Kind)
		assert.NotNil(t, events[0].User)
	})

	t.Run("forbidden without access to the task", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, _, err := (&TaskEvent{TaskID: 1}).ReadAll(s, &user.User{ID: 13}, "", 1, 50)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
}
