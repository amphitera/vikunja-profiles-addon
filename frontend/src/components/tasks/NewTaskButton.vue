<template>
	<template v-if="writableProjects.length > 0">
		<XButton
			class="new-task-button"
			icon="plus"
			variant="secondary"
			:shadow="false"
			@click="open"
		>
			{{ $t('task.newTask.button') }}
		</XButton>
		<Modal
			:enabled="isOpen"
			:overflow="true"
			:aria-label="$t('task.newTask.title')"
			@close="isOpen = false"
		>
			<Card
				:title="$t('task.newTask.title')"
				:shadow="false"
				:padding="false"
				class="has-text-start"
				:loading="saving"
				:show-close="true"
				@close="isOpen = false"
			>
				<form
					class="new-task p-4"
					@submit.prevent="create(false)"
				>
					<input
						ref="titleInput"
						v-model="title"
						class="input new-task__title"
						:placeholder="$t('task.newTask.titlePlaceholder')"
						type="text"
					>

					<div class="new-task__grid">
						<div class="new-task__field">
							<div class="new-task__label">
								{{ $t('task.newTask.assignees') }}
							</div>
							<Multiselect
								v-model="assignees"
								:multiple="true"
								:search-results="foundUsers"
								:show-empty="true"
								label="name"
								:placeholder="$t('task.newTask.assigneesPlaceholder')"
								:autocomplete-enabled="false"
								@search="findUsers"
								@focus="findUsers('')"
							>
								<template #items="{items}">
									<AssigneeList
										:assignees="items"
										can-remove
										@remove="u => assignees = assignees.filter(a => a.id !== u.id)"
									/>
								</template>
								<template #searchResult="{option: user}">
									<User
										:avatar-size="24"
										:show-username="true"
										:user="user"
									/>
								</template>
							</Multiselect>
						</div>
						<div class="new-task__field">
							<div class="new-task__label">
								{{ $t('task.period.title') }}
							</div>
							<TaskPeriod
								:start-date="startDate"
								:end-date="endDate"
								@update="p => { startDate = p.startDate; endDate = p.endDate }"
							/>
						</div>
						<div class="new-task__field">
							<div class="new-task__label">
								{{ $t('task.parent.title') }}
							</div>
							<Multiselect
								v-model="parent"
								:search-results="foundTasks"
								label="title"
								:placeholder="$t('task.parent.searchPlaceholder')"
								@search="findTasks"
							/>
						</div>
						<div
							v-if="writableProjects.length > 1"
							class="new-task__field"
						>
							<div class="new-task__label">
								{{ $t('task.newTask.project') }}
							</div>
							<div class="select is-fullwidth">
								<select v-model="projectId">
									<option
										v-for="p in writableProjects"
										:key="p.id"
										:value="p.id"
									>
										{{ getProjectTitle(p) }}
									</option>
								</select>
							</div>
						</div>
					</div>

					<div class="new-task__field">
						<div class="new-task__label">
							{{ $t('task.newTask.description') }}
						</div>
						<textarea
							v-model="description"
							class="textarea"
							rows="3"
						/>
					</div>

					<div class="new-task__field">
						<input
							ref="fileInput"
							type="file"
							multiple
							class="is-hidden"
							@change="addFiles"
						>
						<BaseButton
							class="new-task__files-button"
							@click="fileInput?.click()"
						>
							<Icon icon="paperclip" />
							{{ $t('task.newTask.addFiles') }}
						</BaseButton>
						<ul
							v-if="files.length > 0"
							class="new-task__files"
						>
							<li
								v-for="(f, i) in files"
								:key="f.name + i"
							>
								{{ f.name }}
								<BaseButton
									:aria-label="$t('misc.delete')"
									@click="files.splice(i, 1)"
								>
									<Icon icon="times" />
								</BaseButton>
							</li>
						</ul>
					</div>
				</form>

				<template #footer>
					<XButton
						variant="secondary"
						:disabled="saving"
						@click="create(true)"
					>
						{{ $t('task.newTask.createAndNext') }}
					</XButton>
					<XButton
						variant="primary"
						icon="plus"
						class="mis-2"
						:loading="saving"
						:disabled="saving"
						@click="create(false)"
					>
						{{ $t('task.newTask.create') }}
					</XButton>
				</template>
			</Card>
		</Modal>
	</template>
</template>

<script setup lang="ts">
import {computed, nextTick, ref, shallowReactive} from 'vue'
import {useI18n} from 'vue-i18n'
import {useStorage} from '@vueuse/core'
import {useRouter} from 'vue-router'

import BaseButton from '@/components/base/BaseButton.vue'
import Card from '@/components/misc/Card.vue'
import Modal from '@/components/misc/Modal.vue'
import User from '@/components/misc/User.vue'
import Multiselect from '@/components/input/Multiselect.vue'
import AssigneeList from '@/components/tasks/partials/AssigneeList.vue'
import TaskPeriod from '@/components/tasks/partials/TaskPeriod.vue'

import TaskService from '@/services/task'
import TaskModel from '@/models/task'
import TaskRelationService from '@/services/taskRelation'
import TaskRelationModel from '@/models/taskRelation'
import ProjectUserService from '@/services/projectUsers'
import {getDisplayName} from '@/models/user'
import {RELATION_KIND} from '@/types/IRelationKind'
import {PERMISSIONS} from '@/constants/permissions'
import {getProjectTitle} from '@/helpers/getProjectTitle'
import {periodFromPreset} from '@/helpers/taskPeriod'
import {uploadFile} from '@/helpers/attachments'
import {isSavedFilter} from '@/services/savedFilter'
import {success, error} from '@/message'

import {useProjectStore} from '@/stores/projects'
import {useBaseStore} from '@/stores/base'
import {useAuthStore} from '@/stores/auth'

import type {ITask} from '@/modelTypes/ITask'
import type {IUser} from '@/modelTypes/IUser'
import type {IProject} from '@/modelTypes/IProject'

const {t} = useI18n({useScope: 'global'})
const router = useRouter()
const projectStore = useProjectStore()
const baseStore = useBaseStore()
const authStore = useAuthStore()

const writableProjects = computed(() => projectStore.projectsArray.filter(p =>
	p.id > 0 && !p.isArchived && !isSavedFilter(p) && (p.maxPermission ?? 0) > PERMISSIONS.READ,
))

const isOpen = ref(false)
const saving = ref(false)
const titleInput = ref<HTMLInputElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

const title = ref('')
const description = ref('')
const projectId = ref<IProject['id']>(0)
const lastProjectId = useStorage<number | null>('newTaskLastProject', null)
const assignees = ref<IUser[]>([])
const startDate = ref<Date | null>(null)
const endDate = ref<Date | null>(null)
const parent = ref<ITask | null>(null)
const files = ref<File[]>([])

const foundUsers = ref<IUser[]>([])
const foundTasks = ref<ITask[]>([])
const projectUserService = shallowReactive(new ProjectUserService())
const taskService = shallowReactive(new TaskService())

function defaultProjectId(): IProject['id'] {
	const isWritable = (id?: number | null) => !!id && writableProjects.value.some(p => p.id === id)
	const current = baseStore.currentProject?.id
	if (isWritable(current)) {
		return current as number
	}
	if (isWritable(lastProjectId.value)) {
		return lastProjectId.value as number
	}
	// The personal default project (the inbox) is invisible to the team, so a shared top-level project wins.
	const personal = authStore.settings.defaultProjectId
	const shared = writableProjects.value.find(p => !p.parentProjectId && p.id !== personal)
	return (shared ?? writableProjects.value[0]).id
}

function resetFields(keepContext: boolean) {
	title.value = ''
	description.value = ''
	files.value = []
	if (!keepContext) {
		assignees.value = []
		parent.value = null
		const period = periodFromPreset(null, 1)
		startDate.value = period.startDate
		endDate.value = period.endDate
	}
}

async function open() {
	projectId.value = defaultProjectId()
	resetFields(false)
	isOpen.value = true
	await nextTick()
	titleInput.value?.focus()
}

async function findUsers(query: string) {
	const users = await projectUserService.getAll({projectId: projectId.value}, {s: query}) as IUser[]
	foundUsers.value = users
		.filter(u => !assignees.value.some(a => a.id === u.id))
		.map(u => ({...u, name: getDisplayName(u)}))
}

async function findTasks(query: string) {
	foundTasks.value = await taskService.getAll(new TaskModel(), {s: query, sort_by: 'done'})
}

function addFiles(e: Event) {
	const input = e.target as HTMLInputElement
	files.value.push(...Array.from(input.files ?? []))
	input.value = ''
}

function toParagraphs(text: string): string {
	const escape = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
	return text.trim().split(/\n{2,}/).map(p => `<p>${escape(p).replace(/\n/g, '<br>')}</p>`).join('')
}

async function create(addAnother: boolean) {
	if (title.value.trim() === '') {
		error({message: t('task.newTask.titleRequired')})
		return
	}
	saving.value = true
	try {
		const model = new TaskModel({
			projectId: projectId.value,
			title: title.value,
			description: description.value.trim() ? toParagraphs(description.value) : '',
			assignees: assignees.value,
		})
		// Set after construction: the constructor's camel-casing turns Date objects into plain objects.
		model.startDate = startDate.value
		model.endDate = endDate.value
		const created = await new TaskService().create(model)
		lastProjectId.value = projectId.value
		if (parent.value) {
			await new TaskRelationService().create(new TaskRelationModel({
				taskId: created.id,
				otherTaskId: parent.value.id,
				relationKind: RELATION_KIND.PARENTTASK,
			}))
		}
		for (const f of files.value) {
			await uploadFile(created.id, f)
		}
		success({message: t('task.newTask.created')}, [{
			title: t('task.newTask.open'),
			callback: () => router.push({name: 'task.detail', params: {id: created.id}}),
		}])
		if (addAnother) {
			resetFields(true)
			await nextTick()
			titleInput.value?.focus()
		} else {
			isOpen.value = false
		}
	} finally {
		saving.value = false
	}
}
</script>

<style lang="scss" scoped>
.new-task-button {
	margin-inline-end: .5rem;
}

.new-task__title {
	font-size: 1.15rem;
	margin-block-end: 1rem;
}

.new-task__grid {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
	gap: 1rem;
	margin-block-end: 1rem;
}

.new-task__field {
	margin-block-end: .75rem;
}

.new-task__label {
	color: var(--grey-500);
	font-size: .85rem;
	margin-block-end: .25rem;
}

.new-task__files-button {
	display: inline-flex;
	align-items: center;
	gap: .35rem;
	color: var(--primary);
}

.new-task__files {
	margin: .5rem 0 0;
	list-style: none;
	padding: 0;

	li {
		display: flex;
		align-items: center;
		gap: .5rem;
	}
}
</style>
