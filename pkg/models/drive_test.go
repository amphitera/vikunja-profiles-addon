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
	"bytes"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

// Project 3 is owned by user 3 (admin); users 1 and 2 may only read it. User 13 has no access.
const driveTestProject = 3

var (
	driveAdmin  = &user.User{ID: 3}
	driveMember = &user.User{ID: 1}
	driveOther  = &user.User{ID: 2}
	driveNobody = &user.User{ID: 13}
)

func driveUpload(t *testing.T, s *xorm.Session, u *user.User, folderID int64, name, content string) *DriveItem {
	item, err := UploadDriveFile(s, u, driveTestProject, folderID, name, bytes.NewReader([]byte(content)), uint64(len(content)))
	require.NoError(t, err)
	return item
}

func TestDrive_FoldersAndFiles(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	folder, err := CreateDriveFolder(s, driveMember, driveTestProject, 0, "Схемы")
	require.NoError(t, err)

	_, err = CreateDriveFolder(s, driveOther, driveTestProject, 0, "схемы")
	assert.True(t, errorIs[ErrDriveNameExists](err), "names are unique regardless of case")

	_, err = CreateDriveFolder(s, driveNobody, driveTestProject, 0, "Чужая")
	assert.True(t, IsErrGenericForbidden(err))

	_, err = CreateDriveFolder(s, driveMember, driveTestProject, 0, "a/b")
	assert.True(t, errorIs[ErrDriveInvalidName](err))

	item := driveUpload(t, s, driveMember, folder.ID, "plan.txt", "v1")
	assert.Equal(t, uint64(2), item.Size)

	again := driveUpload(t, s, driveOther, folder.ID, "PLAN.txt", "version 2")
	assert.Equal(t, item.ID, again.ID, "same name becomes a new version")
	assert.Equal(t, 1, again.Versions)

	listing, err := GetDriveListing(s, driveOther, driveTestProject, folder.ID)
	require.NoError(t, err)
	require.Len(t, listing.Items, 1)
	assert.False(t, listing.Items[0].CanEdit, "only the uploader or an admin edits")
	require.Len(t, listing.Path, 1)

	versions, err := GetDriveItemVersions(s, driveMember, item.ID)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	restored, err := RestoreDriveItemVersion(s, driveMember, item.ID, versions[0].ID)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), restored.Size)

	_, err = GetDriveListing(s, driveNobody, driveTestProject, 0)
	assert.True(t, IsErrGenericForbidden(err))
}

func TestDrive_EditPermissions(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	item := driveUpload(t, s, driveMember, 0, "mine.txt", "x")
	newName := "renamed.txt"

	_, err := UpdateDriveItem(s, driveOther, item.ID, &DriveEntryChange{Name: &newName})
	assert.True(t, IsErrGenericForbidden(err), "another member may not rename")
	assert.True(t, IsErrGenericForbidden(TrashDriveItem(s, driveOther, item.ID)), "another member may not delete")

	renamed, err := UpdateDriveItem(s, driveAdmin, item.ID, &DriveEntryChange{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, newName, renamed.Name)

	require.NoError(t, TrashDriveItem(s, driveMember, item.ID))
	trash, err := GetDriveTrash(s, driveOther, driveTestProject)
	require.NoError(t, err)
	require.Len(t, trash.Items, 1)
	assert.False(t, trash.Items[0].CanEdit)

	_, err = RestoreDriveItem(s, driveOther, item.ID)
	assert.True(t, IsErrGenericForbidden(err), "only who deleted it or an admin restores")
	back, err := RestoreDriveItem(s, driveMember, item.ID)
	require.NoError(t, err)
	assert.True(t, back.DeletedAt.IsZero())
}

func TestDrive_FolderWithForeignFiles(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	folder, err := CreateDriveFolder(s, driveMember, driveTestProject, 0, "Общая")
	require.NoError(t, err)
	driveUpload(t, s, driveOther, folder.ID, "theirs.txt", "y")

	assert.True(t, errorIs[ErrDriveFolderShared](TrashDriveFolder(s, driveMember, folder.ID)))
	require.NoError(t, TrashDriveFolder(s, driveAdmin, folder.ID))

	child, err := CreateDriveFolder(s, driveAdmin, driveTestProject, 0, "Родитель")
	require.NoError(t, err)
	inner, err := CreateDriveFolder(s, driveAdmin, driveTestProject, child.ID, "Внутри")
	require.NoError(t, err)
	_, err = UpdateDriveFolder(s, driveAdmin, child.ID, &DriveEntryChange{FolderID: &inner.ID})
	assert.True(t, errorIs[ErrDriveInvalidMove](err))
}

func TestDrive_TaskAttachmentsShareFiles(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	item := driveUpload(t, s, driveAdmin, 0, "spec.pdf", "pdf")
	attachment, err := AttachDriveItemToTask(s, driveAdmin, 32, item.ID)
	require.NoError(t, err)
	assert.Equal(t, item.FileID, attachment.FileID)

	_, err = AttachDriveItemToTask(s, driveMember, 32, item.ID)
	assert.True(t, IsErrGenericForbidden(err), "read-only members who are not assignees may not attach")

	taskFiles, err := GetDriveTaskFiles(s, driveMember, driveTestProject)
	require.NoError(t, err)
	require.NotEmpty(t, taskFiles)
	assert.Equal(t, int64(32), taskFiles[0].TaskID)

	require.NoError(t, (&TaskAttachment{ID: attachment.ID, TaskID: 32}).Delete(s, driveAdmin))
	require.NoError(t, TrashDriveItem(s, driveAdmin, item.ID))
	require.NoError(t, s.Commit())
	db.AssertExists(t, "files", map[string]interface{}{"id": item.FileID}, false)

	s2 := db.NewSession()
	defer s2.Close()
	require.NoError(t, PurgeDriveTrash(s2, time.Now().Add(31*24*time.Hour)))
	require.NoError(t, s2.Commit())
	db.AssertMissing(t, "drive_items", map[string]interface{}{"id": item.ID})
	db.AssertMissing(t, "files", map[string]interface{}{"id": item.FileID})
}

func errorIs[E error](err error) bool {
	_, ok := err.(E)
	return ok
}
