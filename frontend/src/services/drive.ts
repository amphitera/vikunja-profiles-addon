import type {AxiosProgressEvent} from 'axios'

import {AuthenticatedHTTPFactory, apiV2Url} from '@/helpers/fetcher'
import {useConfigStore} from '@/stores/config'
import type {IUser} from '@/modelTypes/IUser'

export interface DriveFolder {
	id: number
	projectId: number
	parentId: number
	name: string
	createdBy: IUser | null
	created: string
	updated: string
	deletedAt: string | null
	canEdit: boolean
}

export interface DriveItem {
	id: number
	projectId: number
	folderId: number
	name: string
	createdBy: IUser | null
	created: string
	updated: string
	deletedAt: string | null
	size: number
	mime: string
	versions: number
	taskIds: number[]
	canEdit: boolean
}

export interface DriveVersion {
	id: number
	itemId: number
	createdBy: IUser | null
	created: string
	size: number
}

export interface DriveListing {
	projectId: number
	folder: DriveFolder | null
	path: DriveFolder[]
	folders: DriveFolder[]
	items: DriveItem[]
	canUpload: boolean
	isAdmin: boolean
}

export interface DriveTaskAttachment {
	id: number
	taskId: number
	createdBy: IUser | null
	created: string
	file: {id: number, name: string, mime: string, size: number}
}

export interface DriveTaskFiles {
	taskId: number
	title: string
	index: number
	attachments: DriveTaskAttachment[]
}

const http = () => AuthenticatedHTTPFactory()

function camel<T>(data: unknown): T {
	if (Array.isArray(data)) {
		return data.map(d => camel(d)) as T
	}
	if (data && typeof data === 'object') {
		return Object.fromEntries(Object.entries(data as Record<string, unknown>).map(([k, v]) => [
			k.replace(/_([a-z])/g, (_, c) => c.toUpperCase()),
			// Users keep their own field names; they are passed on to user components as they are.
			k === 'created_by' ? v : camel(v),
		])) as T
	}
	return data as T
}

// Big transfers go through the drive URL when the server names one (a separate proxy for files).
function transferUrl(path: string): string {
	const driveUrl = useConfigStore().driveUrl
	if (!driveUrl) {
		return apiV2Url(path)
	}
	return driveUrl.replace(/\/+$/, '') + '/api/v2/' + path
}

export async function getListing(projectId: number, folderId: number): Promise<DriveListing> {
	const {data} = await http().get(apiV2Url(`projects/${projectId}/drive`), {params: {folder: folderId}})
	return camel(data)
}

export async function getTrash(projectId: number): Promise<{folders: DriveFolder[], items: DriveItem[]}> {
	const {data} = await http().get(apiV2Url(`projects/${projectId}/drive/trash`))
	return camel(data)
}

export async function getTaskFiles(projectId: number): Promise<DriveTaskFiles[]> {
	const {data} = await http().get(apiV2Url(`projects/${projectId}/drive/task-files`))
	return camel(data)
}

export async function createFolder(projectId: number, parentId: number, name: string): Promise<DriveFolder> {
	const {data} = await http().post(apiV2Url(`projects/${projectId}/drive/folders`), {name, parent_id: parentId})
	return camel(data)
}

export async function uploadFile(
	projectId: number,
	folderId: number,
	file: File,
	onProgress?: (fraction: number) => void,
): Promise<DriveItem> {
	const form = new FormData()
	form.append('files', file, file.name)
	const {data} = await http().post(transferUrl(`projects/${projectId}/drive/files?folder=${folderId}`), form, {
		// The shared client defaults to JSON and would serialize the form; the browser sets the multipart boundary itself.
		transformRequest: [(body, headers) => {
			headers.delete('Content-Type')
			return body
		}],
		onUploadProgress: (e: AxiosProgressEvent) => onProgress?.(e.total ? e.loaded / e.total : 0),
	})
	return camel<{items: DriveItem[]}>(data).items[0]
}

export async function updateFolder(id: number, change: {name?: string, folderId?: number}): Promise<DriveFolder> {
	const {data} = await http().put(apiV2Url(`drive/folders/${id}`), {name: change.name, folder_id: change.folderId})
	return camel(data)
}

export async function updateItem(id: number, change: {name?: string, folderId?: number}): Promise<DriveItem> {
	const {data} = await http().put(apiV2Url(`drive/files/${id}`), {name: change.name, folder_id: change.folderId})
	return camel(data)
}

export async function trashFolder(id: number) {
	await http().delete(apiV2Url(`drive/folders/${id}`))
}

export async function trashItem(id: number) {
	await http().delete(apiV2Url(`drive/files/${id}`))
}

export async function restoreFolder(id: number) {
	await http().post(apiV2Url(`drive/folders/${id}/restore`))
}

export async function restoreItem(id: number) {
	await http().post(apiV2Url(`drive/files/${id}/restore`))
}

export async function getVersions(id: number): Promise<DriveVersion[]> {
	const {data} = await http().get(apiV2Url(`drive/files/${id}/versions`))
	return camel(data)
}

export async function restoreVersion(id: number, versionId: number) {
	await http().post(apiV2Url(`drive/files/${id}/versions/${versionId}/restore`))
}

// The link is signed by the server and comes back already pointing at the drive URL.
export async function getFileLink(id: number, opts: {versionId?: number, inline?: boolean} = {}): Promise<string> {
	const {data} = await http().post(apiV2Url(`drive/files/${id}/link`), null, {
		params: {version: opts.versionId ?? 0, inline: opts.inline ?? false},
	})
	return data.url
}

export async function getZipLink(projectId: number, folderId: number): Promise<string> {
	const {data} = await http().post(apiV2Url(`projects/${projectId}/drive/zip-link`), null, {params: {folder: folderId}})
	return data.url
}

export async function attachToTask(taskId: number, itemId: number) {
	const {data} = await http().post(apiV2Url(`tasks/${taskId}/attachments/from-drive`), {item_id: itemId})
	return data
}

export function webdavUrl(): string {
	const configStore = useConfigStore()
	const base = configStore.driveUrl || configStore.frontendUrl || window.location.origin
	return base.replace(/\/+$/, '') + '/webdav/'
}

export function startDownload(url: string) {
	const a = document.createElement('a')
	a.href = url
	a.rel = 'noopener'
	document.body.appendChild(a)
	a.click()
	a.remove()
}

export function formatSize(bytes: number): string {
	if (bytes < 1024) {
		return `${bytes} Б`
	}
	const units = ['КБ', 'МБ', 'ГБ']
	let value = bytes / 1024
	let unit = 0
	while (value >= 1024 && unit < units.length - 1) {
		value /= 1024
		unit++
	}
	return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[unit]}`
}
