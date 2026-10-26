<template>
	<div
		class="content loader-container is-max-width-desktop drive"
		:class="{'is-loading': loading}"
		@dragenter.prevent="onDragEnter"
		@dragover.prevent
		@dragleave="onDragLeave"
		@drop.prevent="onDrop"
	>
		<div class="drive__header">
			<h1>{{ $t('drive.title') }}</h1>
			<div
				v-if="drives.length > 1"
				class="select"
			>
				<select
					:value="projectId"
					@change="e => openFolder(Number((e.target as HTMLSelectElement).value), 0)"
				>
					<option
						v-for="p in drives"
						:key="p.id"
						:value="p.id"
					>
						{{ getProjectTitle(p) }}
					</option>
				</select>
			</div>
		</div>

		<nav class="drive__crumbs">
			<BaseButton @click="openFolder(projectId, 0)">
				{{ currentProject ? getProjectTitle(currentProject) : '' }}
			</BaseButton>
			<template v-if="view === 'folder'">
				<template
					v-for="f in listing?.path ?? []"
					:key="f.id"
				>
					<span class="drive__sep">›</span>
					<BaseButton @click="openFolder(projectId, f.id)">
						{{ f.name }}
					</BaseButton>
				</template>
			</template>
			<template v-else>
				<span class="drive__sep">›</span>
				<span>{{ view === 'trash' ? $t('drive.trash') : $t('drive.taskFiles') }}</span>
			</template>
		</nav>

		<div class="drive__toolbar">
			<template v-if="view === 'folder'">
				<XButton
					icon="plus"
					variant="secondary"
					:shadow="false"
					@click="dialog = {kind: 'newFolder', name: ''}"
				>
					{{ $t('drive.newFolder') }}
				</XButton>
				<XButton
					icon="cloud-upload-alt"
					:shadow="false"
					@click="fileInput?.click()"
				>
					{{ $t('drive.upload') }}
				</XButton>
				<input
					ref="fileInput"
					type="file"
					multiple
					class="is-hidden"
					@change="onPick"
				>
				<XButton
					icon="download"
					variant="tertiary"
					:shadow="false"
					@click="downloadZip"
				>
					{{ $t('drive.downloadFolder') }}
				</XButton>
			</template>
			<XButton
				v-else
				icon="arrow-left"
				variant="secondary"
				:shadow="false"
				@click="openFolder(projectId, 0)"
			>
				{{ $t('drive.back') }}
			</XButton>
			<span class="drive__spacer" />
			<XButton
				v-if="view !== 'trash'"
				icon="trash-alt"
				variant="tertiary"
				:shadow="false"
				@click="openTrash"
			>
				{{ $t('drive.trash') }}
			</XButton>
			<XButton
				icon="link"
				variant="tertiary"
				:shadow="false"
				@click="dialog = {kind: 'connect'}"
			>
				{{ $t('drive.connect') }}
			</XButton>
		</div>

		<div
			v-if="uploads.length > 0"
			class="drive__uploads"
		>
			<div
				v-for="u in uploads"
				:key="u.id"
				class="drive__upload"
				:class="{'is-error': u.error}"
			>
				<span class="drive__upload-name">{{ u.name }}</span>
				<span v-if="u.error">{{ u.error }}</span>
				<progress
					v-else
					class="progress is-small is-primary"
					:value="u.progress"
					max="1"
				/>
			</div>
		</div>

		<Card
			:padding="false"
			:has-content="false"
		>
			<!-- Folder view -->
			<table
				v-if="view === 'folder'"
				class="table drive__table is-fullwidth"
			>
				<thead>
					<tr>
						<th>{{ $t('drive.name') }}</th>
						<th class="is-hidden-mobile">
							{{ $t('drive.size') }}
						</th>
						<th class="is-hidden-mobile">
							{{ $t('drive.changed') }}
						</th>
						<th class="is-hidden-mobile">
							{{ $t('drive.author') }}
						</th>
						<th />
					</tr>
				</thead>
				<tbody>
					<tr
						v-if="!listing?.folder"
						class="drive__row is-virtual"
						@click="openTaskFiles"
					>
						<td>
							<Icon
								icon="paperclip"
								class="drive__icon"
							/>
							{{ $t('drive.taskFiles') }}
						</td>
						<td
							class="is-hidden-mobile drive__muted"
							colspan="3"
						>
							{{ $t('drive.taskFilesHint') }}
						</td>
						<td />
					</tr>
					<tr
						v-for="f in listing?.folders ?? []"
						:key="'f' + f.id"
						class="drive__row"
						@click="openFolder(projectId, f.id)"
					>
						<td>
							<Icon
								icon="folder"
								class="drive__icon is-folder"
							/>
							{{ f.name }}
						</td>
						<td class="is-hidden-mobile" />
						<td class="is-hidden-mobile drive__muted">
							{{ formatWhen(f.updated) }}
						</td>
						<td class="is-hidden-mobile drive__muted">
							{{ who(f.createdBy) }}
						</td>
						<td
							class="drive__actions"
							@click.stop
						>
							<template v-if="f.canEdit">
								<BaseButton
									v-tooltip="$t('drive.rename')"
									:aria-label="$t('drive.rename')"
									@click="dialog = {kind: 'rename', folder: f, name: f.name}"
								>
									<Icon icon="pen" />
								</BaseButton>
								<BaseButton
									v-tooltip="$t('drive.move')"
									:aria-label="$t('drive.move')"
									@click="openMove({folder: f})"
								>
									<Icon icon="arrow-up-right-from-square" />
								</BaseButton>
								<BaseButton
									v-tooltip="$t('drive.delete')"
									:aria-label="$t('drive.delete')"
									@click="removeFolder(f)"
								>
									<Icon icon="trash-alt" />
								</BaseButton>
							</template>
						</td>
					</tr>
					<tr
						v-for="i in listing?.items ?? []"
						:key="'i' + i.id"
						class="drive__row"
						@click="preview(i)"
					>
						<td>
							<Icon
								:icon="fileIcon(i.mime)"
								class="drive__icon"
							/>
							{{ i.name }}
							<span
								v-if="i.taskIds.length > 0"
								v-tooltip="$t('drive.attachedTo', {count: i.taskIds.length}, i.taskIds.length)"
								class="drive__badge"
							><Icon icon="paperclip" /> {{ i.taskIds.length }}</span>
						</td>
						<td class="is-hidden-mobile drive__muted">
							{{ formatSize(i.size) }}
						</td>
						<td class="is-hidden-mobile drive__muted">
							{{ formatWhen(i.updated) }}
						</td>
						<td class="is-hidden-mobile drive__muted">
							{{ who(i.createdBy) }}
						</td>
						<td
							class="drive__actions"
							@click.stop
						>
							<BaseButton
								v-tooltip="$t('drive.download')"
								:aria-label="$t('drive.download')"
								@click="download(i)"
							>
								<Icon icon="download" />
							</BaseButton>
							<BaseButton
								v-if="i.versions > 0"
								v-tooltip="$t('drive.versions')"
								:aria-label="$t('drive.versions')"
								@click="openVersions(i)"
							>
								<Icon icon="history" />
							</BaseButton>
							<template v-if="i.canEdit">
								<BaseButton
									v-tooltip="$t('drive.rename')"
									:aria-label="$t('drive.rename')"
									@click="dialog = {kind: 'rename', item: i, name: i.name}"
								>
									<Icon icon="pen" />
								</BaseButton>
								<BaseButton
									v-tooltip="$t('drive.move')"
									:aria-label="$t('drive.move')"
									@click="openMove({item: i})"
								>
									<Icon icon="arrow-up-right-from-square" />
								</BaseButton>
								<BaseButton
									v-tooltip="$t('drive.delete')"
									:aria-label="$t('drive.delete')"
									@click="removeItem(i)"
								>
									<Icon icon="trash-alt" />
								</BaseButton>
							</template>
						</td>
					</tr>
					<tr v-if="listing && listing.folders.length === 0 && listing.items.length === 0">
						<td
							colspan="5"
							class="drive__empty"
						>
							{{ $t('drive.empty') }}
						</td>
					</tr>
				</tbody>
			</table>

			<!-- Task files -->
			<div
				v-else-if="view === 'tasks'"
				class="drive__taskfiles"
			>
				<p
					v-if="taskFiles.length === 0"
					class="drive__empty"
				>
					{{ $t('drive.noTaskFiles') }}
				</p>
				<section
					v-for="tf in taskFiles"
					:key="tf.taskId"
				>
					<RouterLink
						:to="{name: 'task.detail', params: {id: tf.taskId}}"
						class="drive__task-title"
					>
						#{{ tf.index }} {{ tf.title }}
					</RouterLink>
					<ul>
						<li
							v-for="a in tf.attachments"
							:key="a.id"
						>
							<BaseButton @click="downloadAttachment(a)">
								<Icon
									:icon="fileIcon(a.file.mime)"
									class="drive__icon"
								/>
								{{ a.file.name }}
							</BaseButton>
							<span class="drive__muted">{{ formatSize(a.file.size) }} · {{ who(a.createdBy) }}</span>
						</li>
					</ul>
				</section>
			</div>

			<!-- Trash -->
			<table
				v-else
				class="table drive__table is-fullwidth"
			>
				<tbody>
					<tr
						v-for="f in trash.folders"
						:key="'f' + f.id"
					>
						<td>
							<Icon
								icon="folder"
								class="drive__icon is-folder"
							/>
							{{ f.name }}
						</td>
						<td class="drive__muted is-hidden-mobile">
							{{ $t('drive.deletedOn', {date: formatWhen(f.deletedAt)}) }}
						</td>
						<td class="drive__actions">
							<XButton
								v-if="f.canEdit"
								variant="secondary"
								:shadow="false"
								@click="restore({folder: f})"
							>
								{{ $t('drive.restore') }}
							</XButton>
						</td>
					</tr>
					<tr
						v-for="i in trash.items"
						:key="'i' + i.id"
					>
						<td>
							<Icon
								:icon="fileIcon(i.mime)"
								class="drive__icon"
							/>
							{{ i.name }}
						</td>
						<td class="drive__muted is-hidden-mobile">
							{{ $t('drive.deletedOn', {date: formatWhen(i.deletedAt)}) }}
						</td>
						<td class="drive__actions">
							<XButton
								v-if="i.canEdit"
								variant="secondary"
								:shadow="false"
								@click="restore({item: i})"
							>
								{{ $t('drive.restore') }}
							</XButton>
						</td>
					</tr>
					<tr v-if="trash.folders.length === 0 && trash.items.length === 0">
						<td class="drive__empty">
							{{ $t('drive.trashEmpty') }}
						</td>
					</tr>
				</tbody>
			</table>
		</Card>
		<p
			v-if="view === 'trash'"
			class="drive__muted"
		>
			{{ $t('drive.trashHint') }}
		</p>

		<div
			v-if="dragging"
			class="drive__dropzone"
		>
			<Icon icon="cloud-upload-alt" />
			{{ $t('drive.dropHere') }}
		</div>

		<!-- Dialogs -->
		<Modal
			:enabled="dialog !== null"
			:overflow="true"
			:wide="dialog?.kind === 'preview'"
			@close="dialog = null"
		>
			<Card
				v-if="dialog"
				:title="dialogTitle"
				:shadow="false"
				:padding="false"
				class="has-text-start"
				:show-close="true"
				@close="dialog = null"
			>
				<div class="p-4">
					<form
						v-if="dialog.kind === 'newFolder' || dialog.kind === 'rename'"
						@submit.prevent="submitName"
					>
						<input
							v-model="dialog.name"
							v-focus
							class="input"
							type="text"
						>
					</form>

					<div v-else-if="dialog.kind === 'move'">
						<p class="drive__muted">
							{{ $t('drive.moveTo') }}
						</p>
						<ul class="drive__tree">
							<li
								v-for="mt in moveTargets"
								:key="mt.id"
							>
								<BaseButton
									:disabled="mt.disabled"
									:style="{paddingInlineStart: `${mt.depth * 1.25}rem`}"
									@click="submitMove(mt.id)"
								>
									<Icon
										icon="folder"
										class="drive__icon is-folder"
									/>
									{{ mt.name }}
								</BaseButton>
							</li>
						</ul>
					</div>

					<ul
						v-else-if="dialog.kind === 'versions'"
						class="drive__versions"
					>
						<li>
							<strong>{{ $t('drive.currentVersion') }}</strong>
							<span class="drive__muted">{{ formatWhen(dialog.item.updated) }} · {{ formatSize(dialog.item.size) }}</span>
						</li>
						<li
							v-for="v in dialog.versions"
							:key="v.id"
						>
							<span>{{ formatWhen(v.created) }} · {{ formatSize(v.size) }} · {{ who(v.createdBy) }}</span>
							<BaseButton @click="downloadVersion(dialog.item, v)">
								{{ $t('drive.download') }}
							</BaseButton>
							<BaseButton
								v-if="dialog.item.canEdit"
								@click="submitRestoreVersion(dialog.item, v)"
							>
								{{ $t('drive.makeCurrent') }}
							</BaseButton>
						</li>
					</ul>

					<div
						v-else-if="dialog.kind === 'preview'"
						class="drive__preview"
					>
						<img
							v-if="dialog.item.mime.startsWith('image/')"
							:src="dialog.url"
							:alt="dialog.item.name"
						>
						<video
							v-else-if="dialog.item.mime.startsWith('video/')"
							:src="dialog.url"
							controls
						/>
						<iframe
							v-else
							:src="dialog.url"
							:title="dialog.item.name"
						/>
					</div>

					<div
						v-else-if="dialog.kind === 'connect'"
						class="drive__connect"
					>
						<p>{{ $t('drive.connectIntro') }}</p>
						<dl>
							<dt>{{ $t('drive.connectAddress') }}</dt>
							<dd><code>{{ webdavUrl() }}</code></dd>
							<dt>{{ $t('drive.connectLogin') }}</dt>
							<dd><code>{{ authStore.info?.username }}</code></dd>
							<dt>{{ $t('drive.connectPassword') }}</dt>
							<dd>
								<code v-if="dialog.token">{{ dialog.token }}</code>
								<XButton
									v-else
									variant="secondary"
									:shadow="false"
									@click="createToken"
								>
									{{ $t('drive.connectCreatePassword') }}
								</XButton>
							</dd>
						</dl>
						<p
							v-if="dialog.token"
							class="drive__muted"
						>
							{{ $t('drive.connectPasswordOnce') }}
						</p>
						<h3>Windows</h3>
						<p>{{ $t('drive.connectWindows') }}</p>
						<h3>Android</h3>
						<p>{{ $t('drive.connectAndroid') }}</p>
						<h3>iPhone</h3>
						<p>{{ $t('drive.connectIphone') }}</p>
						<h3>Linux</h3>
						<p>{{ $t('drive.connectLinux') }}</p>
					</div>
				</div>

				<template
					v-if="dialog.kind === 'newFolder' || dialog.kind === 'rename' || dialog.kind === 'preview'"
					#footer
				>
					<XButton
						v-if="dialog.kind === 'preview'"
						icon="download"
						@click="download(dialog.item)"
					>
						{{ $t('drive.download') }}
					</XButton>
					<XButton
						v-else
						@click="submitName"
					>
						{{ dialog.kind === 'newFolder' ? $t('drive.create') : $t('drive.save') }}
					</XButton>
				</template>
			</Card>
		</Modal>
	</div>
</template>

<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import type {IconProp} from '@fortawesome/fontawesome-svg-core'

import BaseButton from '@/components/base/BaseButton.vue'
import Card from '@/components/misc/Card.vue'
import Modal from '@/components/misc/Modal.vue'
import {useTitle} from '@/composables/useTitle'
import {getProjectTitle} from '@/helpers/getProjectTitle'
import {getDisplayName} from '@/models/user'
import {isSavedFilter} from '@/services/savedFilter'
import CaldavTokenService from '@/services/caldavToken'
import AttachmentService from '@/services/attachment'
import AttachmentModel from '@/models/attachment'
import {success, error} from '@/message'
import {useProjectStore} from '@/stores/projects'
import {useAuthStore} from '@/stores/auth'
import type {IUser} from '@/modelTypes/IUser'
import * as drive from '@/services/drive'
import {formatSize, webdavUrl, type DriveFolder, type DriveItem, type DriveListing, type DriveTaskFiles, type DriveVersion, type DriveTaskAttachment} from '@/services/drive'

type Dialog =
	| {kind: 'newFolder', name: string}
	| {kind: 'rename', folder?: DriveFolder, item?: DriveItem, name: string}
	| {kind: 'move', folder?: DriveFolder, item?: DriveItem}
	| {kind: 'versions', item: DriveItem, versions: DriveVersion[]}
	| {kind: 'preview', item: DriveItem, url: string}
	| {kind: 'connect', token?: string}

const {t, locale} = useI18n({useScope: 'global'})
useTitle(() => t('drive.title'))
const route = useRoute()
const router = useRouter()
const projectStore = useProjectStore()
const authStore = useAuthStore()

const drives = computed(() => projectStore.projectsArray.filter(p => p.id > 0 && !p.isArchived && !isSavedFilter(p)))
const projectId = computed(() => Number(route.params.projectId) || defaultProjectId())
const currentProject = computed(() => projectStore.projects[projectId.value])
const folderId = computed(() => Number(route.params.folderId) || 0)
const view = computed<'folder' | 'tasks' | 'trash'>(() => {
	const v = route.query.view
	return v === 'tasks' || v === 'trash' ? v : 'folder'
})

function defaultProjectId(): number {
	const personal = authStore.settings.defaultProjectId
	const shared = drives.value.find(p => !p.parentProjectId && p.id !== personal)
	return (shared ?? drives.value[0])?.id ?? 0
}

const loading = ref(false)
const listing = ref<DriveListing | null>(null)
const taskFiles = ref<DriveTaskFiles[]>([])
const trash = ref<{folders: DriveFolder[], items: DriveItem[]}>({folders: [], items: []})
const dialog = ref<Dialog | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

async function load() {
	if (!projectId.value) {
		return
	}
	loading.value = true
	try {
		if (view.value === 'tasks') {
			taskFiles.value = await drive.getTaskFiles(projectId.value)
		} else if (view.value === 'trash') {
			trash.value = await drive.getTrash(projectId.value)
		} else {
			listing.value = await drive.getListing(projectId.value, folderId.value)
		}
	} finally {
		loading.value = false
	}
}

watch(() => [projectId.value, folderId.value, view.value], load, {immediate: true})

function openFolder(project: number, folder: number) {
	router.push({name: 'files', params: {projectId: project, folderId: folder || undefined}})
}

function openTaskFiles() {
	router.push({name: 'files', params: {projectId: projectId.value}, query: {view: 'tasks'}})
}

function openTrash() {
	router.push({name: 'files', params: {projectId: projectId.value}, query: {view: 'trash'}})
}

function who(u: IUser | null) {
	return u ? getDisplayName(u) : ''
}

function formatWhen(date: string | null) {
	if (!date) {
		return ''
	}
	return new Date(date).toLocaleString(locale.value, {day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit'})
}

function fileIcon(mime: string): IconProp {
	if (mime.startsWith('image/')) {
		return 'file-image'
	}
	if (mime === 'application/pdf') {
		return 'file-pdf'
	}
	return 'file'
}

// Uploads
interface Upload {id: number, name: string, progress: number, error: string}
const uploads = ref<Upload[]>([])
let uploadCounter = 0

async function uploadFiles(fileList: File[]) {
	const target = {project: projectId.value, folder: folderId.value}
	for (const file of fileList) {
		uploads.value.push({id: ++uploadCounter, name: file.name, progress: 0, error: ''})
		const live = uploads.value[uploads.value.length - 1]
		try {
			await drive.uploadFile(target.project, target.folder, file, p => { live.progress = p })
			uploads.value = uploads.value.filter(u => u.id !== live.id)
		} catch (e) {
			live.error = (e as {response?: {data?: {message?: string, detail?: string}}})?.response?.data?.detail
				?? (e as {response?: {data?: {message?: string}}})?.response?.data?.message
				?? t('drive.uploadFailed')
		}
	}
	await load()
}

function onPick(e: Event) {
	const input = e.target as HTMLInputElement
	uploadFiles(Array.from(input.files ?? []))
	input.value = ''
}

const dragging = ref(false)
let dragDepth = 0
function onDragEnter(e: DragEvent) {
	if (view.value !== 'folder' || !e.dataTransfer?.types.includes('Files')) {
		return
	}
	dragDepth++
	dragging.value = true
}
function onDragLeave() {
	dragDepth = Math.max(0, dragDepth - 1)
	if (dragDepth === 0) {
		dragging.value = false
	}
}
function onDrop(e: DragEvent) {
	dragDepth = 0
	dragging.value = false
	if (view.value !== 'folder') {
		return
	}
	uploadFiles(Array.from(e.dataTransfer?.files ?? []))
}

// Actions
const dialogTitle = computed(() => {
	switch (dialog.value?.kind) {
		case 'newFolder': return t('drive.newFolder')
		case 'rename': return t('drive.rename')
		case 'move': return t('drive.move')
		case 'versions': return t('drive.versionsOf', {name: dialog.value.item.name})
		case 'preview': return dialog.value.item.name
		case 'connect': return t('drive.connect')
	}
	return ''
})

async function submitName() {
	const d = dialog.value
	if (!d || (d.kind !== 'newFolder' && d.kind !== 'rename')) {
		return
	}
	if (d.kind === 'newFolder') {
		await drive.createFolder(projectId.value, folderId.value, d.name)
	} else if (d.folder) {
		await drive.updateFolder(d.folder.id, {name: d.name})
	} else if (d.item) {
		await drive.updateItem(d.item.id, {name: d.name})
	}
	dialog.value = null
	await load()
}

async function removeFolder(f: DriveFolder) {
	await drive.trashFolder(f.id)
	success({message: t('drive.movedToTrash', {name: f.name})})
	await load()
}

async function removeItem(i: DriveItem) {
	await drive.trashItem(i.id)
	success({message: t('drive.movedToTrash', {name: i.name})})
	await load()
}

async function restore(target: {folder?: DriveFolder, item?: DriveItem}) {
	if (target.folder) {
		await drive.restoreFolder(target.folder.id)
	} else if (target.item) {
		await drive.restoreItem(target.item.id)
	}
	success({message: t('drive.restored')})
	await load()
}

async function download(i: DriveItem) {
	drive.startDownload(await drive.getFileLink(i.id))
}

async function downloadVersion(i: DriveItem, v: DriveVersion) {
	drive.startDownload(await drive.getFileLink(i.id, {versionId: v.id}))
}

async function downloadZip() {
	drive.startDownload(await drive.getZipLink(projectId.value, folderId.value))
}

async function downloadAttachment(a: DriveTaskAttachment) {
	const service = new AttachmentService()
	await service.download(new AttachmentModel({id: a.id, taskId: a.taskId, file: a.file}))
}

const PREVIEWABLE = /^(image\/|video\/|text\/plain|application\/pdf)/
async function preview(i: DriveItem) {
	if (!PREVIEWABLE.test(i.mime)) {
		return download(i)
	}
	dialog.value = {kind: 'preview', item: i, url: await drive.getFileLink(i.id, {inline: true})}
}

async function openVersions(i: DriveItem) {
	dialog.value = {kind: 'versions', item: i, versions: await drive.getVersions(i.id)}
}

async function submitRestoreVersion(i: DriveItem, v: DriveVersion) {
	await drive.restoreVersion(i.id, v.id)
	dialog.value = null
	success({message: t('drive.versionRestored')})
	await load()
}

// Move: a flat, indented list of every folder of the drive.
const moveTargets = ref<{id: number, name: string, depth: number, disabled: boolean}[]>([])

async function openMove(target: {folder?: DriveFolder, item?: DriveItem}) {
	dialog.value = {kind: 'move', ...target}
	const result: {id: number, name: string, depth: number, disabled: boolean}[] = [
		{id: 0, name: currentProject.value ? getProjectTitle(currentProject.value) : '/', depth: 0, disabled: false},
	]
	const walk = async (parentId: number, depth: number, insideMoved: boolean) => {
		const l = await drive.getListing(projectId.value, parentId)
		for (const f of l.folders) {
			const isMoved = insideMoved || f.id === target.folder?.id
			result.push({id: f.id, name: f.name, depth, disabled: isMoved})
			await walk(f.id, depth + 1, isMoved)
		}
	}
	await walk(0, 1, false)
	moveTargets.value = result
}

async function submitMove(targetId: number) {
	const d = dialog.value
	if (!d || d.kind !== 'move') {
		return
	}
	if (d.folder) {
		await drive.updateFolder(d.folder.id, {folderId: targetId})
	} else if (d.item) {
		await drive.updateItem(d.item.id, {folderId: targetId})
	}
	dialog.value = null
	await load()
}

async function createToken() {
	try {
		const token = await new CaldavTokenService().create({}) as unknown as {token: string}
		if (dialog.value?.kind === 'connect') {
			dialog.value = {kind: 'connect', token: token.token}
		}
	} catch {
		error({message: t('drive.connectFailed')})
	}
}
</script>

<style lang="scss" scoped>
.drive {
	position: relative;
	min-block-size: 60vh;
}

.drive__header {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;

	h1 {
		margin: 0;
	}
}

.drive__crumbs {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .35rem;
	margin: 1rem 0 .75rem;
	font-size: 1.05rem;
}

.drive__sep {
	color: var(--grey-400);
}

.drive__toolbar {
	display: flex;
	flex-wrap: wrap;
	gap: .5rem;
	margin-block-end: 1rem;
}

.drive__spacer {
	flex: 1;
}

.drive__uploads {
	margin-block-end: 1rem;
}

.drive__upload {
	display: grid;
	grid-template-columns: minmax(0, 1fr) 12rem;
	gap: 1rem;
	align-items: center;
	font-size: .9rem;

	&.is-error {
		color: var(--danger);
	}

	.progress {
		margin: 0;
	}
}

.drive__upload-name {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.drive__table {
	margin: 0;
	background: transparent;

	th {
		color: var(--grey-500);
		font-weight: 600;
		font-size: .85rem;
	}

	td {
		vertical-align: middle;
	}
}

.drive__row {
	cursor: pointer;

	&:hover {
		background: var(--grey-100);
	}

	&.is-virtual td:first-child {
		font-style: italic;
	}
}

.drive__icon {
	margin-inline-end: .5rem;
	color: var(--grey-500);

	&.is-folder {
		color: var(--primary);
	}
}

.drive__muted {
	color: var(--grey-500);
	font-size: .9rem;
}

.drive__badge {
	margin-inline-start: .5rem;
	font-size: .75rem;
	color: var(--grey-500);
}

.drive__actions {
	text-align: end;
	white-space: nowrap;

	.base-button {
		padding: .25rem .4rem;
		color: var(--grey-500);

		&:hover {
			color: var(--primary);
		}
	}
}

.drive__empty {
	text-align: center;
	color: var(--grey-500);
	padding: 2rem;
}

.drive__taskfiles {
	padding: 1rem;

	section {
		margin-block-end: 1rem;
	}

	ul {
		margin: .25rem 0 0;
		list-style: none;
		padding: 0;
	}

	li {
		display: flex;
		flex-wrap: wrap;
		gap: .75rem;
		align-items: center;
		margin: 0;
	}
}

.drive__task-title {
	font-weight: 600;
}

.drive__dropzone {
	position: absolute;
	inset: 0;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: .75rem;
	font-size: 1.25rem;
	border: 3px dashed var(--primary);
	border-radius: $radius;
	background: hsla(var(--primary-h), var(--primary-s), var(--primary-l), .08);
	pointer-events: none;
	z-index: 10;
}

.drive__tree {
	list-style: none;
	margin: .5rem 0 0;
	padding: 0;
	max-block-size: 50vh;
	overflow: auto;

	li {
		margin: 0;
	}

	.base-button {
		display: block;
		inline-size: 100%;
		text-align: start;
		padding-block: .35rem;

		&:hover:not(:disabled) {
			background: var(--grey-100);
		}

		&:disabled {
			opacity: .4;
		}
	}
}

.drive__versions {
	list-style: none;
	margin: 0;
	padding: 0;

	li {
		display: flex;
		flex-wrap: wrap;
		gap: .75rem;
		align-items: center;
		padding: .4rem 0;
		margin: 0;
		border-block-end: 1px solid var(--grey-200);
	}
}

.drive__preview {
	img,
	video {
		display: block;
		max-inline-size: 100%;
		max-block-size: 75vh;
		margin: 0 auto;
	}

	iframe {
		inline-size: 100%;
		block-size: 75vh;
		border: 0;
	}
}

.drive__connect {
	dl {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: .5rem 1rem;
		align-items: center;
	}

	dd {
		margin: 0;
	}

	code {
		user-select: all;
		word-break: break-all;
	}

	h3 {
		font-size: 1rem;
		margin: 1rem 0 .25rem;
	}
}
</style>
