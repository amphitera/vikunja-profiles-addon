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
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/cron"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// The drive is a project's shared file storage. Files live in the regular file storage,
// so a drive file attached to a task is the same stored file, not a copy.

const (
	driveTrashRetention = 30 * 24 * time.Hour
	driveMaxVersions    = 10
	driveMaxNameLength  = 250
)

type DriveFolder struct {
	ID          int64      `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The unique, numeric id of this folder."`
	ProjectID   int64      `xorm:"bigint not null INDEX" json:"project_id" readOnly:"true" doc:"The project whose drive holds this folder."`
	ParentID    int64      `xorm:"bigint not null default 0 INDEX" json:"parent_id" doc:"The parent folder, 0 for the drive root."`
	Name        string     `xorm:"varchar(250) not null" json:"name" doc:"The folder name."`
	CreatedByID int64      `xorm:"bigint not null" json:"-"`
	CreatedBy   *user.User `xorm:"-" json:"created_by" readOnly:"true" doc:"Who created the folder."`
	Created     time.Time  `xorm:"created not null" json:"created" readOnly:"true" doc:"When the folder was created."`
	Updated     time.Time  `xorm:"updated not null" json:"updated" readOnly:"true" doc:"When the folder was last renamed or moved."`
	DeletedAt   time.Time  `xorm:"DATETIME null INDEX" json:"deleted_at" readOnly:"true" doc:"When the folder was moved to the trash. Empty if it is not in the trash."`
	DeletedByID int64      `xorm:"bigint null" json:"-"`
	CanEdit     bool       `xorm:"-" json:"can_edit" readOnly:"true" doc:"Whether the current user may rename, move or delete this folder."`
}

func (*DriveFolder) TableName() string { return "drive_folders" }

type DriveItem struct {
	ID          int64      `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The unique, numeric id of this file."`
	ProjectID   int64      `xorm:"bigint not null INDEX" json:"project_id" readOnly:"true" doc:"The project whose drive holds this file."`
	FolderID    int64      `xorm:"bigint not null default 0 INDEX" json:"folder_id" doc:"The folder holding the file, 0 for the drive root."`
	Name        string     `xorm:"varchar(250) not null" json:"name" doc:"The file name."`
	FileID      int64      `xorm:"bigint not null INDEX" json:"-"`
	CreatedByID int64      `xorm:"bigint not null" json:"-"`
	CreatedBy   *user.User `xorm:"-" json:"created_by" readOnly:"true" doc:"Who uploaded the file first."`
	Created     time.Time  `xorm:"created not null" json:"created" readOnly:"true" doc:"When the file was first uploaded."`
	Updated     time.Time  `xorm:"updated not null" json:"updated" readOnly:"true" doc:"When the file or its current version last changed."`
	DeletedAt   time.Time  `xorm:"DATETIME null INDEX" json:"deleted_at" readOnly:"true" doc:"When the file was moved to the trash. Empty if it is not in the trash."`
	DeletedByID int64      `xorm:"bigint null" json:"-"`
	Size        uint64     `xorm:"-" json:"size" readOnly:"true" doc:"Size of the current version in bytes."`
	Mime        string     `xorm:"-" json:"mime" readOnly:"true" doc:"Mime type of the current version."`
	Versions    int        `xorm:"-" json:"versions" readOnly:"true" doc:"How many previous versions are kept."`
	TaskIDs     []int64    `xorm:"-" json:"task_ids" readOnly:"true" doc:"Tasks this file is attached to."`
	CanEdit     bool       `xorm:"-" json:"can_edit" readOnly:"true" doc:"Whether the current user may rename, move or delete this file."`
}

func (*DriveItem) TableName() string { return "drive_items" }

type DriveItemVersion struct {
	ID          int64      `xorm:"bigint autoincr not null unique pk" json:"id" readOnly:"true" doc:"The unique, numeric id of this version."`
	ItemID      int64      `xorm:"bigint not null INDEX" json:"item_id" readOnly:"true" doc:"The file this version belongs to."`
	FileID      int64      `xorm:"bigint not null INDEX" json:"-"`
	CreatedByID int64      `xorm:"bigint not null" json:"-"`
	CreatedBy   *user.User `xorm:"-" json:"created_by" readOnly:"true" doc:"Who uploaded this version."`
	Created     time.Time  `xorm:"created not null" json:"created" readOnly:"true" doc:"When this version was replaced by a newer one."`
	Size        uint64     `xorm:"-" json:"size" readOnly:"true" doc:"Size of this version in bytes."`
}

func (*DriveItemVersion) TableName() string { return "drive_item_versions" }

// DriveListing is one folder of a drive.
type DriveListing struct {
	ProjectID int64          `json:"project_id" doc:"The project whose drive this is."`
	Folder    *DriveFolder   `json:"folder" doc:"The listed folder, null for the drive root."`
	Path      []*DriveFolder `json:"path" doc:"The folders from the root down to the listed folder."`
	Folders   []*DriveFolder `json:"folders" doc:"Subfolders, sorted by name."`
	Items     []*DriveItem   `json:"items" doc:"Files, sorted by name."`
	CanUpload bool           `json:"can_upload" doc:"Whether the current user may upload files and create folders here."`
	IsAdmin   bool           `json:"is_admin" doc:"Whether the current user is an admin of the project."`
}

// DriveTaskFiles lists the attachments of one task, shown in the drive as "task files".
type DriveTaskFiles struct {
	TaskID      int64             `json:"task_id" doc:"The task."`
	Title       string            `json:"title" doc:"The task title."`
	Index       int64             `json:"index" doc:"The task's number within its project."`
	Attachments []*TaskAttachment `json:"attachments" doc:"The task's attachments."`
}

const (
	ErrCodeDriveNameExists   = 6910
	ErrCodeDriveNotFound     = 6911
	ErrCodeDriveInvalidName  = 6912
	ErrCodeDriveInvalidMove  = 6913
	ErrCodeDriveFolderShared = 6914
)

type ErrDriveNameExists struct{ Name string }

func (err ErrDriveNameExists) Error() string { return "an entry with this name already exists: " + err.Name }
func (err ErrDriveNameExists) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusConflict, Code: ErrCodeDriveNameExists, Message: "A file or folder with this name already exists here."}
}

type ErrDriveNotFound struct{}

func (err ErrDriveNotFound) Error() string { return "drive entry not found" }
func (err ErrDriveNotFound) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusNotFound, Code: ErrCodeDriveNotFound, Message: "This file or folder does not exist."}
}

type ErrDriveInvalidName struct{}

func (err ErrDriveInvalidName) Error() string { return "invalid drive name" }
func (err ErrDriveInvalidName) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusBadRequest, Code: ErrCodeDriveInvalidName, Message: "The name must not be empty, longer than 250 characters or contain / or \\."}
}

type ErrDriveInvalidMove struct{}

func (err ErrDriveInvalidMove) Error() string { return "cannot move a folder into itself" }
func (err ErrDriveInvalidMove) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusBadRequest, Code: ErrCodeDriveInvalidMove, Message: "A folder cannot be moved into itself or into one of its subfolders."}
}

type ErrDriveFolderShared struct{}

func (err ErrDriveFolderShared) Error() string { return "folder contains entries of other users" }
func (err ErrDriveFolderShared) HTTPError() web.HTTPError {
	return web.HTTPError{HTTPCode: http.StatusForbidden, Code: ErrCodeDriveFolderShared, Message: "This folder contains files of other people. Only a project admin can delete it."}
}

// driveAccess is what a user may do in one project's drive.
type driveAccess struct {
	user    *user.User
	isAdmin bool
}

func (d driveAccess) canEdit(createdByID int64) bool {
	return d.isAdmin || createdByID == d.user.ID
}

// getDriveAccess lets every member who can read the project use its drive — read-only
// members upload too — while only the author or a project admin changes an entry.
func getDriveAccess(s *xorm.Session, a web.Auth, projectID int64) (*driveAccess, error) {
	u, err := user.GetFromAuth(a)
	if err != nil || u == nil || u.ID <= 0 {
		return nil, ErrGenericForbidden{}
	}
	project := &Project{ID: projectID}
	can, _, err := project.CanRead(s, a)
	if err != nil {
		return nil, err
	}
	if !can {
		return nil, ErrGenericForbidden{}
	}
	isAdmin, err := project.IsAdmin(s, a)
	if err != nil {
		return nil, err
	}
	return &driveAccess{user: u, isAdmin: isAdmin}, nil
}

func validDriveName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || len([]rune(name)) > driveMaxNameLength || strings.ContainsAny(name, "/\\") {
		return "", ErrDriveInvalidName{}
	}
	return name, nil
}

func getActiveDriveFolder(s *xorm.Session, projectID, folderID int64) (*DriveFolder, error) {
	f := &DriveFolder{}
	has, err := s.Where(builder.Eq{"id": folderID, "project_id": projectID}).And(builder.IsNull{"deleted_at"}).Get(f)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDriveNotFound{}
	}
	return f, nil
}

func getDriveFolderByID(s *xorm.Session, folderID int64) (*DriveFolder, error) {
	f := &DriveFolder{}
	has, err := s.Where(builder.Eq{"id": folderID}).Get(f)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDriveNotFound{}
	}
	return f, nil
}

func getDriveItemByID(s *xorm.Session, itemID int64) (*DriveItem, error) {
	i := &DriveItem{}
	has, err := s.Where(builder.Eq{"id": itemID}).Get(i)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDriveNotFound{}
	}
	return i, nil
}

// checkDriveParent makes sure a folder id points at a live folder of the project (0 = root).
func checkDriveParent(s *xorm.Session, projectID, folderID int64) error {
	if folderID == 0 {
		return nil
	}
	_, err := getActiveDriveFolder(s, projectID, folderID)
	return err
}

// Names are compared in Go: SQL LOWER() only folds ASCII on SQLite, and names are mostly Cyrillic.
func driveNameTaken(s *xorm.Session, projectID, folderID int64, name string, exceptFolder, exceptItem int64) (bool, error) {
	folder, item, err := FindDriveEntry(s, projectID, folderID, name)
	if err != nil {
		return false, err
	}
	return (folder != nil && folder.ID != exceptFolder) || (item != nil && item.ID != exceptItem), nil
}

// GetDriveListing returns one folder of a project's drive.
func GetDriveListing(s *xorm.Session, a web.Auth, projectID, folderID int64) (*DriveListing, error) {
	access, err := getDriveAccess(s, a, projectID)
	if err != nil {
		return nil, err
	}
	listing := &DriveListing{ProjectID: projectID, Path: []*DriveFolder{}, CanUpload: true, IsAdmin: access.isAdmin}
	if folderID != 0 {
		folder, err := getActiveDriveFolder(s, projectID, folderID)
		if err != nil {
			return nil, err
		}
		listing.Folder = folder
		for f := folder; ; {
			listing.Path = append([]*DriveFolder{f}, listing.Path...)
			if f.ParentID == 0 {
				break
			}
			if f, err = getDriveFolderByID(s, f.ParentID); err != nil {
				return nil, err
			}
		}
	}

	listing.Folders = []*DriveFolder{}
	err = s.Where(builder.Eq{"project_id": projectID, "parent_id": folderID}).And(builder.IsNull{"deleted_at"}).Find(&listing.Folders)
	if err != nil {
		return nil, err
	}
	listing.Items = []*DriveItem{}
	err = s.Where(builder.Eq{"project_id": projectID, "folder_id": folderID}).And(builder.IsNull{"deleted_at"}).Find(&listing.Items)
	if err != nil {
		return nil, err
	}
	if err := enrichDriveEntries(s, access, listing.Folders, listing.Items); err != nil {
		return nil, err
	}
	return listing, nil
}

func enrichDriveEntries(s *xorm.Session, access *driveAccess, folders []*DriveFolder, items []*DriveItem) error {
	userIDs := []int64{}
	fileIDs := []int64{}
	itemIDs := []int64{}
	for _, f := range folders {
		userIDs = append(userIDs, f.CreatedByID)
		f.CanEdit = access.canEdit(f.CreatedByID)
	}
	for _, i := range items {
		userIDs = append(userIDs, i.CreatedByID)
		fileIDs = append(fileIDs, i.FileID)
		itemIDs = append(itemIDs, i.ID)
		i.CanEdit = access.canEdit(i.CreatedByID)
		i.TaskIDs = []int64{}
	}
	users, err := user.GetUsersByIDs(s, userIDs)
	if err != nil {
		return err
	}
	for _, f := range folders {
		f.CreatedBy = users[f.CreatedByID]
	}
	sort.Slice(folders, func(a, b int) bool { return strings.ToLower(folders[a].Name) < strings.ToLower(folders[b].Name) })
	sort.Slice(items, func(a, b int) bool { return strings.ToLower(items[a].Name) < strings.ToLower(items[b].Name) })
	if len(items) == 0 {
		return nil
	}

	fileRows := []*files.File{}
	if err := s.In("id", fileIDs).Find(&fileRows); err != nil {
		return err
	}
	fileByID := map[int64]*files.File{}
	for _, f := range fileRows {
		fileByID[f.ID] = f
	}
	attachments := []*TaskAttachment{}
	if err := s.In("file_id", fileIDs).Find(&attachments); err != nil {
		return err
	}
	versions := []*DriveItemVersion{}
	if err := s.In("item_id", itemIDs).Cols("id", "item_id").Find(&versions); err != nil {
		return err
	}
	versionCount := map[int64]int{}
	for _, v := range versions {
		versionCount[v.ItemID]++
	}
	for _, i := range items {
		i.CreatedBy = users[i.CreatedByID]
		i.Versions = versionCount[i.ID]
		if f := fileByID[i.FileID]; f != nil {
			i.Size = f.Size
			i.Mime = f.Mime
		}
		for _, att := range attachments {
			if att.FileID == i.FileID {
				i.TaskIDs = append(i.TaskIDs, att.TaskID)
			}
		}
	}
	return nil
}

// CreateDriveFolder creates a folder in a project's drive.
func CreateDriveFolder(s *xorm.Session, a web.Auth, projectID, parentID int64, name string) (*DriveFolder, error) {
	access, err := getDriveAccess(s, a, projectID)
	if err != nil {
		return nil, err
	}
	if name, err = validDriveName(name); err != nil {
		return nil, err
	}
	if err := checkDriveParent(s, projectID, parentID); err != nil {
		return nil, err
	}
	taken, err := driveNameTaken(s, projectID, parentID, name, 0, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrDriveNameExists{Name: name}
	}
	folder := &DriveFolder{ProjectID: projectID, ParentID: parentID, Name: name, CreatedByID: access.user.ID}
	if _, err := s.Insert(folder); err != nil {
		return nil, err
	}
	folder.CreatedBy = access.user
	folder.CanEdit = true
	return folder, nil
}

// UploadDriveFile stores a file in a folder. A file with the same name becomes a new
// version of the existing one, so nobody overwrites a colleague's work for good.
func UploadDriveFile(s *xorm.Session, a web.Auth, projectID, folderID int64, name string, content io.ReadSeeker, size uint64) (*DriveItem, error) {
	access, err := getDriveAccess(s, a, projectID)
	if err != nil {
		return nil, err
	}
	if name, err = validDriveName(name); err != nil {
		return nil, err
	}
	if err := checkDriveParent(s, projectID, folderID); err != nil {
		return nil, err
	}

	sameFolder, existing, err := FindDriveEntry(s, projectID, folderID, name)
	if err != nil {
		return nil, err
	}
	if sameFolder != nil {
		return nil, ErrDriveNameExists{Name: name}
	}
	has := existing != nil

	file, err := files.CreateWithSession(s, content, name, size, access.user)
	if err != nil {
		return nil, err
	}

	if !has {
		item := &DriveItem{ProjectID: projectID, FolderID: folderID, Name: name, FileID: file.ID, CreatedByID: access.user.ID}
		if _, err := s.Insert(item); err != nil {
			return nil, err
		}
		return item, enrichDriveEntries(s, access, nil, []*DriveItem{item})
	}

	if _, err := s.Insert(&DriveItemVersion{ItemID: existing.ID, FileID: existing.FileID, CreatedByID: access.user.ID}); err != nil {
		return nil, err
	}
	existing.FileID = file.ID
	if _, err := s.ID(existing.ID).Cols("file_id").Update(existing); err != nil {
		return nil, err
	}
	if err := trimDriveVersions(s, existing.ID); err != nil {
		return nil, err
	}
	return existing, enrichDriveEntries(s, access, nil, []*DriveItem{existing})
}

func trimDriveVersions(s *xorm.Session, itemID int64) error {
	versions := []*DriveItemVersion{}
	if err := s.Where(builder.Eq{"item_id": itemID}).OrderBy("created DESC, id DESC").Find(&versions); err != nil {
		return err
	}
	for i, v := range versions {
		if i < driveMaxVersions {
			continue
		}
		if _, err := s.ID(v.ID).Delete(&DriveItemVersion{}); err != nil {
			return err
		}
		if err := deleteFileIfUnused(s, v.FileID); err != nil {
			return err
		}
	}
	return nil
}

// deleteFileIfUnused removes a stored file once neither the drive nor a task points at it.
func deleteFileIfUnused(s *xorm.Session, fileID int64) error {
	used, err := isFileUsed(s, fileID)
	if err != nil || used {
		return err
	}
	err = (&files.File{ID: fileID}).Delete(s)
	if files.IsErrFileDoesNotExist(err) {
		return nil
	}
	return err
}

func isFileUsed(s *xorm.Session, fileID int64) (bool, error) {
	for _, bean := range []interface{}{&TaskAttachment{}, &DriveItem{}, &DriveItemVersion{}} {
		n, err := s.Where(builder.Eq{"file_id": fileID}).Count(bean)
		if err != nil || n > 0 {
			return n > 0, err
		}
	}
	return false, nil
}

// fileUsedByDrive tells task attachments not to delete a file the drive still shows.
func fileUsedByDrive(s *xorm.Session, fileID int64) (bool, error) {
	for _, bean := range []interface{}{&DriveItem{}, &DriveItemVersion{}} {
		n, err := s.Where(builder.Eq{"file_id": fileID}).Count(bean)
		if err != nil || n > 0 {
			return n > 0, err
		}
	}
	return false, nil
}

// fileUsedByOtherAttachments keeps a file attached to several tasks alive until the last one goes.
func fileUsedByOtherAttachments(s *xorm.Session, fileID, attachmentID int64) (bool, error) {
	n, err := s.Where(builder.Eq{"file_id": fileID}).And(builder.Neq{"id": attachmentID}).Count(&TaskAttachment{})
	return n > 0, err
}

// DriveEntryChange renames and/or moves a file or folder.
type DriveEntryChange struct {
	Name     *string `json:"name,omitempty" doc:"The new name. Leave out to keep the name."`
	FolderID *int64  `json:"folder_id,omitempty" doc:"The folder to move into, 0 for the drive root. Leave out to keep the place."`
}

func UpdateDriveItem(s *xorm.Session, a web.Auth, itemID int64, change *DriveEntryChange) (*DriveItem, error) {
	item, err := getDriveItemByID(s, itemID)
	if err != nil || !item.DeletedAt.IsZero() {
		return nil, ErrDriveNotFound{}
	}
	access, err := getDriveAccess(s, a, item.ProjectID)
	if err != nil {
		return nil, err
	}
	if !access.canEdit(item.CreatedByID) {
		return nil, ErrGenericForbidden{}
	}
	if change.Name != nil {
		if item.Name, err = validDriveName(*change.Name); err != nil {
			return nil, err
		}
	}
	if change.FolderID != nil {
		if err := checkDriveParent(s, item.ProjectID, *change.FolderID); err != nil {
			return nil, err
		}
		item.FolderID = *change.FolderID
	}
	taken, err := driveNameTaken(s, item.ProjectID, item.FolderID, item.Name, 0, item.ID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrDriveNameExists{Name: item.Name}
	}
	if _, err := s.ID(item.ID).Cols("name", "folder_id").Update(item); err != nil {
		return nil, err
	}
	return item, enrichDriveEntries(s, access, nil, []*DriveItem{item})
}

func UpdateDriveFolder(s *xorm.Session, a web.Auth, folderID int64, change *DriveEntryChange) (*DriveFolder, error) {
	folder, err := getDriveFolderByID(s, folderID)
	if err != nil || !folder.DeletedAt.IsZero() {
		return nil, ErrDriveNotFound{}
	}
	access, err := getDriveAccess(s, a, folder.ProjectID)
	if err != nil {
		return nil, err
	}
	if !access.canEdit(folder.CreatedByID) {
		return nil, ErrGenericForbidden{}
	}
	if change.Name != nil {
		if folder.Name, err = validDriveName(*change.Name); err != nil {
			return nil, err
		}
	}
	if change.FolderID != nil && *change.FolderID != folder.ParentID {
		target := *change.FolderID
		if err := checkDriveParent(s, folder.ProjectID, target); err != nil {
			return nil, err
		}
		for id := target; id != 0; {
			if id == folder.ID {
				return nil, ErrDriveInvalidMove{}
			}
			parent, err := getDriveFolderByID(s, id)
			if err != nil {
				return nil, err
			}
			id = parent.ParentID
		}
		folder.ParentID = target
	}
	taken, err := driveNameTaken(s, folder.ProjectID, folder.ParentID, folder.Name, folder.ID, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrDriveNameExists{Name: folder.Name}
	}
	if _, err := s.ID(folder.ID).Cols("name", "parent_id").Update(folder); err != nil {
		return nil, err
	}
	return folder, enrichDriveEntries(s, access, []*DriveFolder{folder}, nil)
}

// TrashDriveItem moves a file to the trash.
func TrashDriveItem(s *xorm.Session, a web.Auth, itemID int64) error {
	item, err := getDriveItemByID(s, itemID)
	if err != nil || !item.DeletedAt.IsZero() {
		return ErrDriveNotFound{}
	}
	access, err := getDriveAccess(s, a, item.ProjectID)
	if err != nil {
		return err
	}
	if !access.canEdit(item.CreatedByID) {
		return ErrGenericForbidden{}
	}
	_, err = s.ID(item.ID).Cols("deleted_at", "deleted_by_id").Update(&DriveItem{DeletedAt: time.Now(), DeletedByID: access.user.ID})
	return err
}

// TrashDriveFolder moves a folder with everything in it to the trash. Members may only do
// that when everything inside is theirs.
func TrashDriveFolder(s *xorm.Session, a web.Auth, folderID int64) error {
	folder, err := getDriveFolderByID(s, folderID)
	if err != nil || !folder.DeletedAt.IsZero() {
		return ErrDriveNotFound{}
	}
	access, err := getDriveAccess(s, a, folder.ProjectID)
	if err != nil {
		return err
	}
	if !access.canEdit(folder.CreatedByID) {
		return ErrGenericForbidden{}
	}
	if !access.isAdmin {
		foreign, err := driveFolderHasForeignEntries(s, folder.ID, access.user.ID)
		if err != nil {
			return err
		}
		if foreign {
			return ErrDriveFolderShared{}
		}
	}
	_, err = s.ID(folder.ID).Cols("deleted_at", "deleted_by_id").Update(&DriveFolder{DeletedAt: time.Now(), DeletedByID: access.user.ID})
	return err
}

func driveFolderHasForeignEntries(s *xorm.Session, folderID, userID int64) (bool, error) {
	queue := []int64{folderID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		n, err := s.Where(builder.Eq{"folder_id": id}).And(builder.IsNull{"deleted_at"}).And(builder.Neq{"created_by_id": userID}).Count(&DriveItem{})
		if err != nil || n > 0 {
			return n > 0, err
		}
		children := []*DriveFolder{}
		if err := s.Where(builder.Eq{"parent_id": id}).And(builder.IsNull{"deleted_at"}).Find(&children); err != nil {
			return false, err
		}
		for _, c := range children {
			if c.CreatedByID != userID {
				return true, nil
			}
			queue = append(queue, c.ID)
		}
	}
	return false, nil
}

// DriveTrash lists the trashed entries of a project's drive, newest first.
type DriveTrash struct {
	Folders []*DriveFolder `json:"folders" doc:"Trashed folders."`
	Items   []*DriveItem   `json:"items" doc:"Trashed files."`
}

func GetDriveTrash(s *xorm.Session, a web.Auth, projectID int64) (*DriveTrash, error) {
	access, err := getDriveAccess(s, a, projectID)
	if err != nil {
		return nil, err
	}
	trash := &DriveTrash{Folders: []*DriveFolder{}, Items: []*DriveItem{}}
	if err := s.Where(builder.Eq{"project_id": projectID}).And(builder.NotNull{"deleted_at"}).OrderBy("deleted_at DESC").Find(&trash.Folders); err != nil {
		return nil, err
	}
	if err := s.Where(builder.Eq{"project_id": projectID}).And(builder.NotNull{"deleted_at"}).OrderBy("deleted_at DESC").Find(&trash.Items); err != nil {
		return nil, err
	}
	if err := enrichDriveEntries(s, access, trash.Folders, trash.Items); err != nil {
		return nil, err
	}
	// In the trash, "edit" means restore: the person who deleted it or an admin.
	for _, f := range trash.Folders {
		f.CanEdit = access.isAdmin || f.DeletedByID == access.user.ID
	}
	for _, i := range trash.Items {
		i.CanEdit = access.isAdmin || i.DeletedByID == access.user.ID
	}
	sort.Slice(trash.Folders, func(a, b int) bool { return trash.Folders[a].DeletedAt.After(trash.Folders[b].DeletedAt) })
	sort.Slice(trash.Items, func(a, b int) bool { return trash.Items[a].DeletedAt.After(trash.Items[b].DeletedAt) })
	return trash, nil
}

// restoreTarget puts an entry back where it was, or into the root when that folder is gone.
func restoreTarget(s *xorm.Session, projectID, folderID int64) int64 {
	if folderID == 0 {
		return 0
	}
	if _, err := getActiveDriveFolder(s, projectID, folderID); err != nil {
		return 0
	}
	return folderID
}

// freeName appends " (2)", " (3)", … until the name is free in the folder.
func freeName(s *xorm.Session, projectID, folderID int64, name string, exceptFolder, exceptItem int64) (string, error) {
	base, ext := name, ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base, ext = name[:dot], name[dot:]
	}
	candidate := name
	for n := 2; ; n++ {
		taken, err := driveNameTaken(s, projectID, folderID, candidate, exceptFolder, exceptItem)
		if err != nil || !taken {
			return candidate, err
		}
		candidate = base + " (" + strconv.Itoa(n) + ")" + ext
	}
}


func RestoreDriveItem(s *xorm.Session, a web.Auth, itemID int64) (*DriveItem, error) {
	item, err := getDriveItemByID(s, itemID)
	if err != nil || item.DeletedAt.IsZero() {
		return nil, ErrDriveNotFound{}
	}
	access, err := getDriveAccess(s, a, item.ProjectID)
	if err != nil {
		return nil, err
	}
	if !access.isAdmin && item.DeletedByID != access.user.ID {
		return nil, ErrGenericForbidden{}
	}
	item.FolderID = restoreTarget(s, item.ProjectID, item.FolderID)
	if item.Name, err = freeName(s, item.ProjectID, item.FolderID, item.Name, 0, item.ID); err != nil {
		return nil, err
	}
	_, err = s.ID(item.ID).Cols("deleted_at", "deleted_by_id", "folder_id", "name").Update(&DriveItem{FolderID: item.FolderID, Name: item.Name})
	if err != nil {
		return nil, err
	}
	item.DeletedAt = time.Time{}
	return item, enrichDriveEntries(s, access, nil, []*DriveItem{item})
}

func RestoreDriveFolder(s *xorm.Session, a web.Auth, folderID int64) (*DriveFolder, error) {
	folder, err := getDriveFolderByID(s, folderID)
	if err != nil || folder.DeletedAt.IsZero() {
		return nil, ErrDriveNotFound{}
	}
	access, err := getDriveAccess(s, a, folder.ProjectID)
	if err != nil {
		return nil, err
	}
	if !access.isAdmin && folder.DeletedByID != access.user.ID {
		return nil, ErrGenericForbidden{}
	}
	folder.ParentID = restoreTarget(s, folder.ProjectID, folder.ParentID)
	if folder.Name, err = freeName(s, folder.ProjectID, folder.ParentID, folder.Name, folder.ID, 0); err != nil {
		return nil, err
	}
	_, err = s.ID(folder.ID).Cols("deleted_at", "deleted_by_id", "parent_id", "name").Update(&DriveFolder{ParentID: folder.ParentID, Name: folder.Name})
	if err != nil {
		return nil, err
	}
	folder.DeletedAt = time.Time{}
	return folder, enrichDriveEntries(s, access, []*DriveFolder{folder}, nil)
}

// GetDriveItemVersions lists the previous versions of a file, newest first.
func GetDriveItemVersions(s *xorm.Session, a web.Auth, itemID int64) ([]*DriveItemVersion, error) {
	item, err := getDriveItemByID(s, itemID)
	if err != nil {
		return nil, err
	}
	if _, err := getDriveAccess(s, a, item.ProjectID); err != nil {
		return nil, err
	}
	versions := []*DriveItemVersion{}
	if err := s.Where(builder.Eq{"item_id": itemID}).OrderBy("created DESC, id DESC").Find(&versions); err != nil {
		return nil, err
	}
	userIDs := []int64{}
	fileIDs := []int64{}
	for _, v := range versions {
		userIDs = append(userIDs, v.CreatedByID)
		fileIDs = append(fileIDs, v.FileID)
	}
	users, err := user.GetUsersByIDs(s, userIDs)
	if err != nil {
		return nil, err
	}
	fileRows := []*files.File{}
	if len(fileIDs) > 0 {
		if err := s.In("id", fileIDs).Find(&fileRows); err != nil {
			return nil, err
		}
	}
	for _, v := range versions {
		v.CreatedBy = users[v.CreatedByID]
		for _, f := range fileRows {
			if f.ID == v.FileID {
				v.Size = f.Size
			}
		}
	}
	return versions, nil
}

// RestoreDriveItemVersion makes an old version current again; the current one becomes a version.
func RestoreDriveItemVersion(s *xorm.Session, a web.Auth, itemID, versionID int64) (*DriveItem, error) {
	item, err := getDriveItemByID(s, itemID)
	if err != nil || !item.DeletedAt.IsZero() {
		return nil, ErrDriveNotFound{}
	}
	access, err := getDriveAccess(s, a, item.ProjectID)
	if err != nil {
		return nil, err
	}
	if !access.canEdit(item.CreatedByID) {
		return nil, ErrGenericForbidden{}
	}
	version := &DriveItemVersion{}
	has, err := s.Where(builder.Eq{"id": versionID, "item_id": itemID}).Get(version)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrDriveNotFound{}
	}
	if _, err := s.ID(version.ID).Cols("file_id", "created_by_id").Update(&DriveItemVersion{FileID: item.FileID, CreatedByID: access.user.ID}); err != nil {
		return nil, err
	}
	item.FileID = version.FileID
	if _, err := s.ID(item.ID).Cols("file_id").Update(item); err != nil {
		return nil, err
	}
	return item, enrichDriveEntries(s, access, nil, []*DriveItem{item})
}

// GetDriveFile returns the stored file behind a drive file (or one of its versions) after
// checking that the user may read it. The returned file has its content opened.
func GetDriveFile(s *xorm.Session, a web.Auth, itemID, versionID int64) (*files.File, string, error) {
	item, err := getDriveItemByID(s, itemID)
	if err != nil {
		return nil, "", err
	}
	if _, err := getDriveAccess(s, a, item.ProjectID); err != nil {
		return nil, "", err
	}
	fileID := item.FileID
	if versionID != 0 {
		version := &DriveItemVersion{}
		has, err := s.Where(builder.Eq{"id": versionID, "item_id": itemID}).Get(version)
		if err != nil {
			return nil, "", err
		}
		if !has {
			return nil, "", ErrDriveNotFound{}
		}
		fileID = version.FileID
	}
	f := &files.File{ID: fileID}
	if err := f.LoadFileMetaByID(s); err != nil {
		return nil, "", err
	}
	return f, item.Name, nil
}

// GetDriveTaskFiles lists the attachments of the project's tasks for the "task files" folder.
func GetDriveTaskFiles(s *xorm.Session, a web.Auth, projectID int64) ([]*DriveTaskFiles, error) {
	if _, err := getDriveAccess(s, a, projectID); err != nil {
		return nil, err
	}
	tasks := []*Task{}
	err := s.Where(builder.Eq{"project_id": projectID}).
		And(builder.In("id", builder.Select("task_id").From("task_attachments"))).
		OrderBy("id DESC").Find(&tasks)
	if err != nil {
		return nil, err
	}
	result := []*DriveTaskFiles{}
	if len(tasks) == 0 {
		return result, nil
	}
	taskIDs := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		taskIDs = append(taskIDs, t.ID)
	}
	attachments := []*TaskAttachment{}
	if err := s.In("task_id", taskIDs).OrderBy("id ASC").Find(&attachments); err != nil {
		return nil, err
	}
	fileIDs := []int64{}
	userIDs := []int64{}
	for _, att := range attachments {
		fileIDs = append(fileIDs, att.FileID)
		userIDs = append(userIDs, att.CreatedByID)
	}
	fileRows := []*files.File{}
	if err := s.In("id", fileIDs).Find(&fileRows); err != nil {
		return nil, err
	}
	users, err := user.GetUsersByIDs(s, userIDs)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		entry := &DriveTaskFiles{TaskID: t.ID, Title: t.Title, Index: t.Index, Attachments: []*TaskAttachment{}}
		for _, att := range attachments {
			if att.TaskID != t.ID {
				continue
			}
			att.CreatedBy = users[att.CreatedByID]
			for _, f := range fileRows {
				if f.ID == att.FileID {
					att.File = f
				}
			}
			entry.Attachments = append(entry.Attachments, att)
		}
		result = append(result, entry)
	}
	return result, nil
}

// AttachDriveItemToTask attaches a drive file to a task without copying it.
func AttachDriveItemToTask(s *xorm.Session, a web.Auth, taskID, itemID int64) (*TaskAttachment, error) {
	item, err := getDriveItemByID(s, itemID)
	if err != nil || !item.DeletedAt.IsZero() {
		return nil, ErrDriveNotFound{}
	}
	if _, err := getDriveAccess(s, a, item.ProjectID); err != nil {
		return nil, err
	}
	attachment := &TaskAttachment{TaskID: taskID, FileID: item.FileID}
	can, err := attachment.CanCreate(s, a)
	if err != nil {
		return nil, err
	}
	if !can {
		return nil, ErrGenericForbidden{}
	}
	attachment.CreatedByID = a.GetID()
	if _, err := s.Insert(attachment); err != nil {
		return nil, err
	}
	attachment.File = &files.File{ID: item.FileID}
	if err := attachment.File.LoadFileMetaByID(s); err != nil {
		return nil, err
	}
	return attachment, nil
}

// RegisterDriveTrashCron removes trashed drive entries once their retention is over.
func RegisterDriveTrashCron() {
	err := cron.Schedule("30 3 * * *", func() {
		s := db.NewSession()
		defer s.Close()
		if err := PurgeDriveTrash(s, time.Now()); err != nil {
			_ = s.Rollback()
			log.Errorf("Could not purge the drive trash: %s", err)
			return
		}
		if err := s.Commit(); err != nil {
			log.Errorf("Could not purge the drive trash: %s", err)
		}
	})
	if err != nil {
		log.Errorf("Could not register drive trash cron: %s", err.Error())
	}
}

// PurgeDriveTrash deletes entries that have been in the trash longer than the retention.
func PurgeDriveTrash(s *xorm.Session, now time.Time) error {
	cutoff := now.Add(-driveTrashRetention)
	folders := []*DriveFolder{}
	if err := s.Where(builder.Lt{"deleted_at": cutoff}).Find(&folders); err != nil {
		return err
	}
	for _, f := range folders {
		if err := purgeDriveFolder(s, f.ID); err != nil {
			return err
		}
	}
	items := []*DriveItem{}
	if err := s.Where(builder.Lt{"deleted_at": cutoff}).Find(&items); err != nil {
		return err
	}
	for _, i := range items {
		if err := purgeDriveItem(s, i); err != nil {
			return err
		}
	}
	return nil
}

func purgeDriveFolder(s *xorm.Session, folderID int64) error {
	children := []*DriveFolder{}
	if err := s.Where(builder.Eq{"parent_id": folderID}).Find(&children); err != nil {
		return err
	}
	for _, c := range children {
		if err := purgeDriveFolder(s, c.ID); err != nil {
			return err
		}
	}
	items := []*DriveItem{}
	if err := s.Where(builder.Eq{"folder_id": folderID}).Find(&items); err != nil {
		return err
	}
	for _, i := range items {
		if err := purgeDriveItem(s, i); err != nil {
			return err
		}
	}
	_, err := s.ID(folderID).Delete(&DriveFolder{})
	return err
}

func purgeDriveItem(s *xorm.Session, item *DriveItem) error {
	versions := []*DriveItemVersion{}
	if err := s.Where(builder.Eq{"item_id": item.ID}).Find(&versions); err != nil {
		return err
	}
	if _, err := s.Where(builder.Eq{"item_id": item.ID}).Delete(&DriveItemVersion{}); err != nil {
		return err
	}
	if _, err := s.ID(item.ID).Delete(&DriveItem{}); err != nil {
		return err
	}
	for _, v := range versions {
		if err := deleteFileIfUnused(s, v.FileID); err != nil {
			return err
		}
	}
	return deleteFileIfUnused(s, item.FileID)
}

// WalkDriveFolder calls fn for every live file below a folder (0 = whole drive) with its
// path relative to that folder. Used for ZIP downloads.
func WalkDriveFolder(s *xorm.Session, projectID, folderID int64, fn func(path string, item *DriveItem) error) error {
	var walk func(id int64, prefix string) error
	walk = func(id int64, prefix string) error {
		items := []*DriveItem{}
		if err := s.Where(builder.Eq{"project_id": projectID, "folder_id": id}).And(builder.IsNull{"deleted_at"}).Find(&items); err != nil {
			return err
		}
		for _, i := range items {
			if err := fn(prefix+i.Name, i); err != nil {
				return err
			}
		}
		folders := []*DriveFolder{}
		if err := s.Where(builder.Eq{"project_id": projectID, "parent_id": id}).And(builder.IsNull{"deleted_at"}).Find(&folders); err != nil {
			return err
		}
		for _, f := range folders {
			if err := walk(f.ID, prefix+f.Name+"/"); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(folderID, "")
}

// CheckDriveFolderReadable verifies access before a ZIP of a folder is handed out.
func CheckDriveFolderReadable(s *xorm.Session, a web.Auth, projectID, folderID int64) (string, error) {
	if _, err := getDriveAccess(s, a, projectID); err != nil {
		return "", err
	}
	if folderID == 0 {
		project, err := GetProjectSimpleByID(s, projectID)
		if err != nil {
			return "", err
		}
		return project.Title, nil
	}
	folder, err := getActiveDriveFolder(s, projectID, folderID)
	if err != nil {
		return "", err
	}
	return folder.Name, nil
}

// CheckDriveAccess tells whether the user may use a project's drive.
func CheckDriveAccess(s *xorm.Session, a web.Auth, projectID int64) error {
	_, err := getDriveAccess(s, a, projectID)
	return err
}

// FindDriveEntry looks up a live folder or file by name in a folder (0 = root), case-insensitively.
// Both results are nil when nothing matches.
func FindDriveEntry(s *xorm.Session, projectID, folderID int64, name string) (*DriveFolder, *DriveItem, error) {
	folders := []*DriveFolder{}
	if err := s.Where(builder.Eq{"project_id": projectID, "parent_id": folderID}).And(builder.IsNull{"deleted_at"}).Find(&folders); err != nil {
		return nil, nil, err
	}
	for _, f := range folders {
		if strings.EqualFold(f.Name, name) {
			return f, nil, nil
		}
	}
	items := []*DriveItem{}
	if err := s.Where(builder.Eq{"project_id": projectID, "folder_id": folderID}).And(builder.IsNull{"deleted_at"}).Find(&items); err != nil {
		return nil, nil, err
	}
	for _, i := range items {
		if strings.EqualFold(i.Name, name) {
			return nil, i, nil
		}
	}
	return nil, nil, nil
}

// ListDriveChildren returns the live content of a folder without permission checks;
// callers check CheckDriveAccess first. Items come with size and mime.
func ListDriveChildren(s *xorm.Session, projectID, folderID int64) ([]*DriveFolder, []*DriveItem, error) {
	folders := []*DriveFolder{}
	if err := s.Where(builder.Eq{"project_id": projectID, "parent_id": folderID}).And(builder.IsNull{"deleted_at"}).Find(&folders); err != nil {
		return nil, nil, err
	}
	items := []*DriveItem{}
	if err := s.Where(builder.Eq{"project_id": projectID, "folder_id": folderID}).And(builder.IsNull{"deleted_at"}).Find(&items); err != nil {
		return nil, nil, err
	}
	if len(items) == 0 {
		return folders, items, nil
	}
	fileIDs := make([]int64, 0, len(items))
	for _, i := range items {
		fileIDs = append(fileIDs, i.FileID)
	}
	fileRows := []*files.File{}
	if err := s.In("id", fileIDs).Find(&fileRows); err != nil {
		return nil, nil, err
	}
	for _, i := range items {
		for _, f := range fileRows {
			if f.ID == i.FileID {
				i.Size = f.Size
				i.Mime = f.Mime
			}
		}
	}
	return folders, items, nil
}

// DriveItemSize fills in size and mime of a single file.
func DriveItemSize(s *xorm.Session, item *DriveItem) error {
	f := &files.File{ID: item.FileID}
	if err := f.LoadFileMetaByID(s); err != nil {
		return err
	}
	item.Size = f.Size
	item.Mime = f.Mime
	return nil
}
