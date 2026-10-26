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

package migration

import (
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/builder"
	"xorm.io/xorm"
)

type taskEvents20261026120000 struct {
	ID       int64     `xorm:"bigint autoincr not null unique pk"`
	TaskID   int64     `xorm:"bigint not null INDEX"`
	UserID   int64     `xorm:"bigint not null"`
	Kind     string    `xorm:"varchar(50) not null"`
	OldValue string    `xorm:"text null"`
	NewValue string    `xorm:"text null"`
	Created  time.Time `xorm:"created not null"`
}

func (taskEvents20261026120000) TableName() string {
	return "task_events"
}

type tasks20261026120000 struct {
	ID      int64     `xorm:"bigint autoincr not null unique pk"`
	DueDate time.Time `xorm:"DATETIME INDEX null 'due_date'"`
	EndDate time.Time `xorm:"DATETIME INDEX null 'end_date'"`
}

func (tasks20261026120000) TableName() string {
	return "tasks"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261026120000",
		Description: "Add task_events and fill due_date from end_date",
		Migrate: func(tx *xorm.Engine) error {
			if err := tx.Sync(taskEvents20261026120000{}); err != nil {
				return err
			}

			// From now on the due date follows the end date (see Task.syncDueDateWithEndDate);
			// existing tasks only ever had end dates set.
			tasks := []*tasks20261026120000{}
			err := tx.Where(builder.And(builder.NotNull{"end_date"}, builder.IsNull{"due_date"})).Find(&tasks)
			if err != nil {
				return err
			}
			for _, t := range tasks {
				_, err = tx.Where(builder.Eq{"id": t.ID}).
					Cols("due_date").
					Update(&tasks20261026120000{DueDate: t.EndDate})
				if err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(taskEvents20261026120000{})
		},
	})
}
