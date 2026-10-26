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

// Package drivedav serves the project drives over WebDAV, so they can be mounted as a
// network drive or synced with WebDAV clients. The top level lists the projects the
// user can read; below that are the folders and files of each project's drive.
package drivedav

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"github.com/labstack/echo/v5"
	"golang.org/x/net/webdav"
	"xorm.io/xorm"
)

const Prefix = "/webdav"

var lockSystem = webdav.NewMemLS()

// Handler serves WebDAV for the user BasicAuth put on the context.
func Handler(c *echo.Context) error {
	u, ok := c.Get("userBasicAuth").(*user.User)
	if !ok || u == nil {
		return echo.ErrUnauthorized
	}
	h := &webdav.Handler{
		Prefix:     Prefix,
		FileSystem: &driveFS{auth: u},
		LockSystem: lockSystem,
		Logger: func(r *http.Request, err error) {
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Debugf("webdav %s %s: %v", r.Method, r.URL.Path, err)
			}
		},
	}
	h.ServeHTTP(c.Response(), c.Request())
	return nil
}

type driveFS struct {
	auth web.Auth
}

// node is what a path points at.
type node struct {
	root      bool
	projectID int64
	project   *models.Project
	folder    *models.DriveFolder // nil at a project's root
	item      *models.DriveItem
}

func (n *node) folderID() int64 {
	if n.folder == nil {
		return 0
	}
	return n.folder.ID
}

func withSession(fn func(s *xorm.Session) error) error {
	s := db.NewSession()
	defer s.Close()
	if err := fn(s); err != nil {
		_ = s.Rollback()
		return toOSError(err)
	}
	return s.Commit()
}

// toOSError maps domain errors onto the os errors the webdav package turns into status codes.
func toOSError(err error) error {
	var notFound models.ErrDriveNotFound
	var exists models.ErrDriveNameExists
	var forbidden models.ErrGenericForbidden
	var shared models.ErrDriveFolderShared
	switch {
	case errors.As(err, &notFound):
		return os.ErrNotExist
	case errors.As(err, &exists):
		return os.ErrExist
	case errors.As(err, &forbidden), errors.As(err, &shared):
		return os.ErrPermission
	}
	return err
}

func projectDirName(p *models.Project, all []*models.Project) string {
	name := strings.NewReplacer("/", "-", "\\", "-").Replace(p.Title)
	for _, other := range all {
		if other.ID != p.ID && strings.EqualFold(other.Title, p.Title) {
			return name + " (" + strconv.FormatInt(p.ID, 10) + ")"
		}
	}
	return name
}

func (d *driveFS) projects(s *xorm.Session) ([]*models.Project, error) {
	result, _, _, err := (&models.Project{}).ReadAll(s, d.auth, "", 1, -1)
	if err != nil {
		return nil, err
	}
	all, ok := result.([]*models.Project)
	if !ok {
		return nil, errors.New("unexpected project list type")
	}
	projects := make([]*models.Project, 0, len(all))
	for _, p := range all {
		if p.ID > 0 {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

func splitPath(name string) []string {
	clean := strings.Trim(path.Clean("/"+name), "/")
	if clean == "" {
		return nil
	}
	return strings.Split(clean, "/")
}

func (d *driveFS) resolve(s *xorm.Session, name string) (*node, error) {
	parts := splitPath(name)
	if len(parts) == 0 {
		return &node{root: true}, nil
	}
	projects, err := d.projects(s)
	if err != nil {
		return nil, err
	}
	var project *models.Project
	for _, p := range projects {
		if strings.EqualFold(projectDirName(p, projects), parts[0]) {
			project = p
			break
		}
	}
	if project == nil {
		return nil, os.ErrNotExist
	}
	if err := models.CheckDriveAccess(s, d.auth, project.ID); err != nil {
		return nil, os.ErrNotExist
	}
	n := &node{projectID: project.ID, project: project}
	for _, part := range parts[1:] {
		if n.item != nil {
			return nil, os.ErrNotExist
		}
		folder, item, err := models.FindDriveEntry(s, project.ID, n.folderID(), part)
		if err != nil {
			return nil, err
		}
		switch {
		case folder != nil:
			n.folder = folder
		case item != nil:
			if err := models.DriveItemSize(s, item); err != nil {
				return nil, err
			}
			n.item = item
		default:
			return nil, os.ErrNotExist
		}
	}
	return n, nil
}

// resolveParent resolves the folder a new entry goes into and returns the entry's name.
func (d *driveFS) resolveParent(s *xorm.Session, name string) (*node, string, error) {
	parts := splitPath(name)
	if len(parts) < 2 {
		// Nothing can be created at the top level, which lists projects.
		return nil, "", os.ErrPermission
	}
	parent, err := d.resolve(s, strings.Join(parts[:len(parts)-1], "/"))
	if err != nil {
		return nil, "", err
	}
	if parent.root || parent.item != nil {
		return nil, "", os.ErrPermission
	}
	return parent, parts[len(parts)-1], nil
}

func (d *driveFS) Mkdir(_ context.Context, name string, _ os.FileMode) error {
	return withSession(func(s *xorm.Session) error {
		parent, base, err := d.resolveParent(s, name)
		if err != nil {
			return err
		}
		_, err = models.CreateDriveFolder(s, d.auth, parent.projectID, parent.folderID(), base)
		return err
	})
}

func (d *driveFS) OpenFile(_ context.Context, name string, flag int, _ os.FileMode) (webdav.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0 {
		var parent *node
		var base string
		err := withSession(func(s *xorm.Session) (err error) {
			parent, base, err = d.resolveParent(s, name)
			if err != nil {
				return err
			}
			// Existing folders cannot be overwritten by a file.
			folder, _, err := models.FindDriveEntry(s, parent.projectID, parent.folderID(), base)
			if err == nil && folder != nil {
				return os.ErrExist
			}
			return err
		})
		if err != nil {
			return nil, err
		}
		tmp, err := os.CreateTemp("", "vikunja-webdav-*")
		if err != nil {
			return nil, err
		}
		return &writeFile{File: tmp, fs: d, projectID: parent.projectID, folderID: parent.folderID(), name: base}, nil
	}

	var n *node
	if err := withSession(func(s *xorm.Session) (err error) {
		n, err = d.resolve(s, name)
		return err
	}); err != nil {
		return nil, err
	}
	if n.item == nil {
		return &dirFile{fs: d, node: n, name: path.Base("/" + name)}, nil
	}
	f := &files.File{ID: n.item.FileID}
	if err := f.LoadFileByID(); err != nil {
		return nil, err
	}
	rs, ok := f.File.(io.ReadSeeker)
	if !ok {
		_ = f.File.Close()
		return nil, errors.New("file storage is not seekable")
	}
	return &readFile{ReadSeeker: rs, closer: f.File, info: itemInfo(n.item)}, nil
}

func (d *driveFS) RemoveAll(_ context.Context, name string) error {
	return withSession(func(s *xorm.Session) error {
		n, err := d.resolve(s, name)
		if err != nil {
			return err
		}
		switch {
		case n.item != nil:
			return models.TrashDriveItem(s, d.auth, n.item.ID)
		case n.folder != nil:
			return models.TrashDriveFolder(s, d.auth, n.folder.ID)
		}
		return os.ErrPermission
	})
}

func (d *driveFS) Rename(_ context.Context, oldName, newName string) error {
	return withSession(func(s *xorm.Session) error {
		n, err := d.resolve(s, oldName)
		if err != nil {
			return err
		}
		parent, base, err := d.resolveParent(s, newName)
		if err != nil {
			return err
		}
		if parent.projectID != n.projectID {
			return os.ErrPermission
		}
		folderID := parent.folderID()
		change := &models.DriveEntryChange{Name: &base, FolderID: &folderID}
		switch {
		case n.item != nil:
			_, err = models.UpdateDriveItem(s, d.auth, n.item.ID, change)
		case n.folder != nil:
			_, err = models.UpdateDriveFolder(s, d.auth, n.folder.ID, change)
		default:
			err = os.ErrPermission
		}
		return err
	})
}

func (d *driveFS) Stat(_ context.Context, name string) (os.FileInfo, error) {
	var info os.FileInfo
	err := withSession(func(s *xorm.Session) error {
		n, err := d.resolve(s, name)
		if err != nil {
			return err
		}
		info = nodeInfo(n, path.Base("/"+name))
		return nil
	})
	return info, err
}

type fileInfo struct {
	name    string
	size    int64
	dir     bool
	modTime time.Time
}

func (f *fileInfo) Name() string       { return f.name }
func (f *fileInfo) Size() int64        { return f.size }
func (f *fileInfo) ModTime() time.Time { return f.modTime }
func (f *fileInfo) IsDir() bool        { return f.dir }
func (f *fileInfo) Sys() any           { return nil }
func (f *fileInfo) Mode() fs.FileMode {
	if f.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

func itemInfo(i *models.DriveItem) *fileInfo {
	// #nosec G115 - file sizes stay far below int64 max.
	return &fileInfo{name: i.Name, size: int64(i.Size), modTime: i.Updated}
}

func nodeInfo(n *node, name string) os.FileInfo {
	switch {
	case n.item != nil:
		return itemInfo(n.item)
	case n.folder != nil:
		return &fileInfo{name: n.folder.Name, dir: true, modTime: n.folder.Updated}
	case n.project != nil:
		return &fileInfo{name: name, dir: true, modTime: n.project.Updated}
	}
	return &fileInfo{name: "/", dir: true, modTime: time.Now()}
}

type dirFile struct {
	fs   *driveFS
	node *node
	name string
	read bool
}

func (f *dirFile) Close() error                           { return nil }
func (f *dirFile) Read([]byte) (int, error)               { return 0, os.ErrInvalid }
func (f *dirFile) Seek(int64, int) (int64, error)         { return 0, os.ErrInvalid }
func (f *dirFile) Write([]byte) (int, error)              { return 0, os.ErrPermission }
func (f *dirFile) Stat() (os.FileInfo, error)             { return nodeInfo(f.node, f.name), nil }
func (f *dirFile) Readdir(count int) ([]os.FileInfo, error) {
	if f.read {
		if count > 0 {
			return nil, io.EOF
		}
		return []os.FileInfo{}, nil
	}
	f.read = true
	infos := []os.FileInfo{}
	err := withSession(func(s *xorm.Session) error {
		if f.node.root {
			projects, err := f.fs.projects(s)
			if err != nil {
				return err
			}
			for _, p := range projects {
				infos = append(infos, &fileInfo{name: projectDirName(p, projects), dir: true, modTime: p.Updated})
			}
			return nil
		}
		folders, items, err := models.ListDriveChildren(s, f.node.projectID, f.node.folderID())
		if err != nil {
			return err
		}
		for _, folder := range folders {
			infos = append(infos, &fileInfo{name: folder.Name, dir: true, modTime: folder.Updated})
		}
		for _, item := range items {
			infos = append(infos, itemInfo(item))
		}
		return nil
	})
	return infos, err
}

type readFile struct {
	io.ReadSeeker
	closer io.Closer
	info   os.FileInfo
}

func (f *readFile) Close() error                          { return f.closer.Close() }
func (f *readFile) Write([]byte) (int, error)             { return 0, os.ErrPermission }
func (f *readFile) Readdir(int) ([]os.FileInfo, error)    { return nil, os.ErrInvalid }
func (f *readFile) Stat() (os.FileInfo, error)            { return f.info, nil }

// writeFile buffers an upload on disk and stores it in the drive when the client is done.
type writeFile struct {
	*os.File
	fs        *driveFS
	projectID int64
	folderID  int64
	name      string
}

func (f *writeFile) Readdir(int) ([]os.FileInfo, error) { return nil, os.ErrInvalid }

func (f *writeFile) Stat() (os.FileInfo, error) {
	st, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &fileInfo{name: f.name, size: st.Size(), modTime: time.Now()}, nil
}

func (f *writeFile) Close() error {
	defer os.Remove(f.File.Name())
	defer f.File.Close()
	st, err := f.File.Stat()
	if err != nil {
		return err
	}
	if _, err := f.File.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return withSession(func(s *xorm.Session) error {
		// #nosec G115 - Stat sizes are non-negative.
		_, err := models.UploadDriveFile(s, f.fs.auth, f.projectID, f.folderID, f.name, f.File, uint64(st.Size()))
		return err
	})
}
