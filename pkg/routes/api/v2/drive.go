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
	"archive/zip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humaecho"
	"xorm.io/xorm"
)

// Signed links let the browser fetch big files straight from the (possibly separate)
// file host without holding them in memory or sending the session token along.
const driveLinkLifetime = 10 * time.Minute

type driveLink struct {
	FileID    int64  `json:"f,omitempty"`
	Name      string `json:"n,omitempty"`
	ProjectID int64  `json:"p,omitempty"`
	FolderID  int64  `json:"d,omitempty"`
	Zip       bool   `json:"z,omitempty"`
	Inline    bool   `json:"i,omitempty"`
	Expires   int64  `json:"e"`
}

func driveSign(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(config.ServiceSecret.GetString()+"/drive-links"))
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func makeDriveLink(l driveLink) string {
	l.Expires = time.Now().Add(driveLinkLifetime).Unix()
	payload, _ := json.Marshal(l)
	token := base64.RawURLEncoding.EncodeToString(payload) + "." + driveSign(payload)
	base := config.ServiceDrivePublicURL.GetString()
	if base == "" {
		base = config.ServicePublicURL.GetString()
	}
	return strings.TrimSuffix(base, "/") + "/api/v2/drive/download/" + token
}

func parseDriveLink(token string) (*driveLink, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	if !hmac.Equal([]byte(driveSign(payload)), []byte(parts[1])) {
		return nil, errors.New("bad signature")
	}
	l := &driveLink{}
	if err := json.Unmarshal(payload, l); err != nil {
		return nil, err
	}
	if time.Now().Unix() > l.Expires {
		return nil, errors.New("expired")
	}
	return l, nil
}

type driveLinkBody struct {
	Body struct {
		URL string `json:"url" doc:"A link valid for 10 minutes that downloads without further authentication."`
	}
}

func RegisterDriveRoutes(api huma.API) {
	tags := []string{"drive"}
	// Each upload request carries one file; the frontend sends files one by one for progress.
	// #nosec G115 - configured value won't exceed int64 max in practice.
	maxUpload := (int64(config.GetMaxFileSizeInMBytes()) + 2) * 1024 * 1024

	Register(api, huma.Operation{OperationID: "drive-list", Summary: "List a drive folder",
		Description: "Returns the subfolders and files of one folder of a project's drive (folder=0 or none for the root), with the path from the root. Everyone who can read the project can use its drive.",
		Method:      http.MethodGet, Path: "/projects/{project}/drive", Tags: tags}, driveList)
	Register(api, huma.Operation{OperationID: "drive-trash", Summary: "List the drive trash",
		Description: "Returns the trashed files and folders of a project's drive. Trashed entries are removed for good after 30 days. can_edit tells whether the current user may restore an entry: the person who deleted it or a project admin.",
		Method:      http.MethodGet, Path: "/projects/{project}/drive/trash", Tags: tags}, driveTrash)
	Register(api, huma.Operation{OperationID: "drive-task-files", Summary: "List the project's task attachments",
		Description: "Returns the attachments of all tasks of the project, grouped by task. The drive shows them as a read-only \"task files\" folder.",
		Method:      http.MethodGet, Path: "/projects/{project}/drive/task-files", Tags: tags}, driveTaskFiles)
	Register(api, huma.Operation{OperationID: "drive-folders-create", Summary: "Create a drive folder",
		Description: "Creates a folder. Names must be unique within their folder (case-insensitive).",
		Method:      http.MethodPost, Path: "/projects/{project}/drive/folders", Tags: tags}, driveFolderCreate)
	Register(api, huma.Operation{OperationID: "drive-files-upload", Summary: "Upload a file to the drive",
		Description: "Uploads files via multipart/form-data under the \"files\" field into a folder. A file with the same name as an existing one becomes its new version; up to 10 previous versions are kept.",
		Method:      http.MethodPost, Path: "/projects/{project}/drive/files", Tags: tags, MaxBodyBytes: maxUpload}, driveUpload)
	Register(api, huma.Operation{OperationID: "drive-zip-link", Summary: "Get a ZIP download link for a folder",
		Description: "Returns a short-lived link that downloads the folder (folder=0 for the whole drive) with everything in it as a ZIP archive.",
		Method:      http.MethodPost, Path: "/projects/{project}/drive/zip-link", Tags: tags}, driveZipLink)

	Register(api, huma.Operation{OperationID: "drive-folders-update", Summary: "Rename or move a drive folder",
		Description: "Renames and/or moves a folder. Only its creator or a project admin may do this.",
		Method:      http.MethodPut, Path: "/drive/folders/{folder}", Tags: tags}, driveFolderUpdate)
	Register(api, huma.Operation{OperationID: "drive-folders-delete", Summary: "Move a drive folder to the trash",
		Description: "Moves a folder with its content to the trash. Its creator may only do this when everything inside is theirs; a project admin always may.",
		Method:      http.MethodDelete, Path: "/drive/folders/{folder}", Tags: tags}, driveFolderDelete)
	Register(api, huma.Operation{OperationID: "drive-folders-restore", Summary: "Restore a drive folder from the trash",
		Description: "Puts the folder back where it was, or into the root if that folder is gone. A clashing name gets a number appended.",
		Method:      http.MethodPost, Path: "/drive/folders/{folder}/restore", Tags: tags}, driveFolderRestore)

	Register(api, huma.Operation{OperationID: "drive-files-update", Summary: "Rename or move a drive file",
		Description: "Renames and/or moves a file. Only its uploader or a project admin may do this.",
		Method:      http.MethodPut, Path: "/drive/files/{file}", Tags: tags}, driveItemUpdate)
	Register(api, huma.Operation{OperationID: "drive-files-delete", Summary: "Move a drive file to the trash",
		Description: "Only its uploader or a project admin may do this. Tasks the file is attached to keep it.",
		Method:      http.MethodDelete, Path: "/drive/files/{file}", Tags: tags}, driveItemDelete)
	Register(api, huma.Operation{OperationID: "drive-files-restore", Summary: "Restore a drive file from the trash",
		Description: "Puts the file back where it was, or into the root if that folder is gone. A clashing name gets a number appended.",
		Method:      http.MethodPost, Path: "/drive/files/{file}/restore", Tags: tags}, driveItemRestore)
	Register(api, huma.Operation{OperationID: "drive-files-versions", Summary: "List the previous versions of a drive file",
		Method: http.MethodGet, Path: "/drive/files/{file}/versions", Tags: tags}, driveItemVersions)
	Register(api, huma.Operation{OperationID: "drive-files-versions-restore", Summary: "Make a previous version current",
		Description: "The current version becomes a previous version, so nothing is lost.",
		Method:      http.MethodPost, Path: "/drive/files/{file}/versions/{version}/restore", Tags: tags}, driveItemVersionRestore)
	Register(api, huma.Operation{OperationID: "drive-files-link", Summary: "Get a download link for a drive file",
		Description: "Returns a short-lived link to the file, or to one of its versions. inline=true asks the browser to show it instead of saving it.",
		Method:      http.MethodPost, Path: "/drive/files/{file}/link", Tags: tags}, driveItemLink)

	Register(api, huma.Operation{OperationID: "task-attachments-from-drive", Summary: "Attach a drive file to a task",
		Description: "Attaches a file from the drive without copying it. Same permission as uploading an attachment.",
		Method:      http.MethodPost, Path: "/tasks/{task}/attachments/from-drive", Tags: tags}, driveAttachToTask)

	Register(api, huma.Operation{OperationID: "drive-download", Summary: "Download through a signed link",
		Description: "Streams the file or ZIP archive a link from the link endpoints points at. Needs no authentication; the link expires after 10 minutes.",
		Method:      http.MethodGet, Path: "/drive/download/{token}", Tags: tags,
		Security: []map[string][]string{},
		Responses: map[string]*huma.Response{"200": {Description: "The file bytes.", Content: map[string]*huma.MediaType{
			"application/octet-stream": {Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"}},
		}}},
	}, driveDownload)
}

func init() { AddRouteRegistrar(RegisterDriveRoutes) }

// inSession runs fn in a session and commits when it succeeds.
func inSession[T any](ctx context.Context, fn func(s *xorm.Session, a web.Auth) (T, error)) (T, error) {
	var zero T
	a, err := authFromCtx(ctx)
	if err != nil {
		return zero, err
	}
	s := db.NewSession()
	defer s.Close()
	result, err := fn(s, a)
	if err != nil {
		_ = s.Rollback()
		return zero, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return zero, translateDomainError(err)
	}
	return result, nil
}

type driveListingBody struct{ Body *models.DriveListing }

func driveList(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
	FolderID  int64 `query:"folder" doc:"The folder to list, 0 or empty for the root."`
}) (*driveListingBody, error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*driveListingBody, error) {
		l, err := models.GetDriveListing(s, a, in.ProjectID, in.FolderID)
		return &driveListingBody{Body: l}, err
	})
}

type driveTrashBody struct{ Body *models.DriveTrash }

func driveTrash(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
}) (*driveTrashBody, error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*driveTrashBody, error) {
		t, err := models.GetDriveTrash(s, a, in.ProjectID)
		return &driveTrashBody{Body: t}, err
	})
}

type driveTaskFilesBody struct{ Body []*models.DriveTaskFiles }

func driveTaskFiles(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
}) (*driveTaskFilesBody, error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*driveTaskFilesBody, error) {
		t, err := models.GetDriveTaskFiles(s, a, in.ProjectID)
		return &driveTaskFilesBody{Body: t}, err
	})
}

func driveFolderCreate(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
	Body      struct {
		Name     string `json:"name" doc:"The folder name."`
		ParentID int64  `json:"parent_id" doc:"The parent folder, 0 for the root."`
	}
}) (*singleBody[models.DriveFolder], error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*singleBody[models.DriveFolder], error) {
		f, err := models.CreateDriveFolder(s, a, in.ProjectID, in.Body.ParentID, in.Body.Name)
		return &singleBody[models.DriveFolder]{Body: f}, err
	})
}

type driveUploadBody struct {
	Body struct {
		Items  []*models.DriveItem `json:"items" doc:"The stored files."`
		Errors []string            `json:"errors" doc:"One message per file that could not be stored."`
	}
}

func driveUpload(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
	FolderID  int64 `query:"folder" doc:"The folder to upload into, 0 or empty for the root."`
	RawBody   huma.MultipartFormFiles[struct {
		Files []huma.FormFile `form:"files" required:"true" doc:"The files to upload."`
	}]
}) (*driveUploadBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	out := &driveUploadBody{}
	out.Body.Items = []*models.DriveItem{}
	out.Body.Errors = []string{}
	for _, f := range in.RawBody.Data().Files {
		s := db.NewSession()
		// #nosec G115 - multipart sizes are non-negative.
		item, err := models.UploadDriveFile(s, a, in.ProjectID, in.FolderID, f.Filename, f, uint64(f.Size))
		if err != nil {
			_ = s.Rollback()
			s.Close()
			if len(in.RawBody.Data().Files) == 1 {
				return nil, translateDomainError(err)
			}
			out.Body.Errors = append(out.Body.Errors, f.Filename+": "+err.Error())
			continue
		}
		if err := s.Commit(); err != nil {
			s.Close()
			return nil, translateDomainError(err)
		}
		s.Close()
		out.Body.Items = append(out.Body.Items, item)
	}
	return out, nil
}

func driveZipLink(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
	FolderID  int64 `query:"folder" doc:"The folder to pack, 0 or empty for the whole drive."`
}) (*driveLinkBody, error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*driveLinkBody, error) {
		name, err := models.CheckDriveFolderReadable(s, a, in.ProjectID, in.FolderID)
		if err != nil {
			return nil, err
		}
		out := &driveLinkBody{}
		out.Body.URL = makeDriveLink(driveLink{Zip: true, ProjectID: in.ProjectID, FolderID: in.FolderID, Name: name + ".zip"})
		return out, nil
	})
}

type driveChangeInput struct {
	ID   int64 `path:"folder"`
	Body models.DriveEntryChange
}

func driveFolderUpdate(ctx context.Context, in *driveChangeInput) (*singleBody[models.DriveFolder], error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*singleBody[models.DriveFolder], error) {
		f, err := models.UpdateDriveFolder(s, a, in.ID, &in.Body)
		return &singleBody[models.DriveFolder]{Body: f}, err
	})
}

func driveFolderDelete(ctx context.Context, in *struct {
	ID int64 `path:"folder"`
}) (*emptyBody, error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*emptyBody, error) {
		return &emptyBody{}, models.TrashDriveFolder(s, a, in.ID)
	})
}

func driveFolderRestore(ctx context.Context, in *struct {
	ID int64 `path:"folder"`
}) (*singleBody[models.DriveFolder], error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*singleBody[models.DriveFolder], error) {
		f, err := models.RestoreDriveFolder(s, a, in.ID)
		return &singleBody[models.DriveFolder]{Body: f}, err
	})
}

func driveItemUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"file"`
	Body models.DriveEntryChange
}) (*singleBody[models.DriveItem], error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*singleBody[models.DriveItem], error) {
		i, err := models.UpdateDriveItem(s, a, in.ID, &in.Body)
		return &singleBody[models.DriveItem]{Body: i}, err
	})
}

func driveItemDelete(ctx context.Context, in *struct {
	ID int64 `path:"file"`
}) (*emptyBody, error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*emptyBody, error) {
		return &emptyBody{}, models.TrashDriveItem(s, a, in.ID)
	})
}

func driveItemRestore(ctx context.Context, in *struct {
	ID int64 `path:"file"`
}) (*singleBody[models.DriveItem], error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*singleBody[models.DriveItem], error) {
		i, err := models.RestoreDriveItem(s, a, in.ID)
		return &singleBody[models.DriveItem]{Body: i}, err
	})
}

type driveVersionsBody struct{ Body []*models.DriveItemVersion }

func driveItemVersions(ctx context.Context, in *struct {
	ID int64 `path:"file"`
}) (*driveVersionsBody, error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*driveVersionsBody, error) {
		v, err := models.GetDriveItemVersions(s, a, in.ID)
		return &driveVersionsBody{Body: v}, err
	})
}

func driveItemVersionRestore(ctx context.Context, in *struct {
	ID        int64 `path:"file"`
	VersionID int64 `path:"version"`
}) (*singleBody[models.DriveItem], error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*singleBody[models.DriveItem], error) {
		i, err := models.RestoreDriveItemVersion(s, a, in.ID, in.VersionID)
		return &singleBody[models.DriveItem]{Body: i}, err
	})
}

func driveItemLink(ctx context.Context, in *struct {
	ID        int64 `path:"file"`
	VersionID int64 `query:"version" doc:"A previous version to link to instead of the current one."`
	Inline    bool  `query:"inline" doc:"Show the file in the browser instead of saving it."`
}) (*driveLinkBody, error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*driveLinkBody, error) {
		f, name, err := models.GetDriveFile(s, a, in.ID, in.VersionID)
		if err != nil {
			return nil, err
		}
		out := &driveLinkBody{}
		out.Body.URL = makeDriveLink(driveLink{FileID: f.ID, Name: name, Inline: in.Inline})
		return out, nil
	})
}

func driveAttachToTask(ctx context.Context, in *struct {
	TaskID int64 `path:"task"`
	Body   struct {
		ItemID int64 `json:"item_id" doc:"The drive file to attach."`
	}
}) (*singleBody[models.TaskAttachment], error) {
	return inSession(ctx, func(s *xorm.Session, a web.Auth) (*singleBody[models.TaskAttachment], error) {
		att, err := models.AttachDriveItemToTask(s, a, in.TaskID, in.Body.ItemID)
		return &singleBody[models.TaskAttachment]{Body: att}, err
	})
}

func driveDownload(_ context.Context, in *struct {
	Token string `path:"token"`
}) (*huma.StreamResponse, error) {
	link, err := parseDriveLink(in.Token)
	if err != nil {
		return nil, huma.Error404NotFound("This link is invalid or has expired.")
	}
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		c := humaecho.Unwrap(hctx)
		w, r := (*c).Response(), (*c).Request()
		if link.Zip {
			writeDriveZip(w, link)
			return
		}
		writeDriveFile(w, r, link)
	}}, nil
}

func writeDriveFile(w http.ResponseWriter, r *http.Request, link *driveLink) {
	s := db.NewSession()
	f := &files.File{ID: link.FileID}
	err := f.LoadFileMetaByID(s)
	s.Close()
	if err == nil {
		err = f.LoadFileByID()
	}
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.File.Close()

	disposition := "attachment"
	if link.Inline {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": link.Name}))
	contentType := f.Mime
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-cache")
	if seeker, ok := f.File.(io.ReadSeeker); ok {
		http.ServeContent(w, r, link.Name, f.Created, seeker)
		return
	}
	w.Header().Set("Content-Length", strconv.FormatUint(f.Size, 10))
	_, _ = io.Copy(w, f.File)
}

func writeDriveZip(w http.ResponseWriter, link *driveLink) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": link.Name}))
	zw := zip.NewWriter(w)
	defer zw.Close()

	s := db.NewSession()
	defer s.Close()
	err := models.WalkDriveFolder(s, link.ProjectID, link.FolderID, func(path string, item *models.DriveItem) error {
		f := &files.File{ID: item.FileID}
		if err := f.LoadFileByID(); err != nil {
			log.Errorf("drive zip: could not open file %d: %v", item.FileID, err)
			return nil
		}
		defer f.File.Close()
		entry, err := zw.CreateHeader(&zip.FileHeader{Name: path, Method: zip.Deflate, Modified: item.Updated})
		if err != nil {
			return err
		}
		_, err = io.Copy(entry, f.File)
		return err
	})
	if err != nil {
		log.Errorf("drive zip for project %d folder %d failed: %v", link.ProjectID, link.FolderID, err)
	}
}
