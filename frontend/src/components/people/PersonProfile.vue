<template>
	<div class="person-profile">
		<header class="person-header">
			<User
				:user="user"
				:avatar-size="compact ? 56 : 72"
				:show-username="false"
			/>
			<div class="person-heading">
				<h2 class="person-name">
					{{ displayName }}
				</h2>
				<p class="person-username">
					@{{ user.username }}
				</p>
				<form
					v-if="editingRole"
					class="role-form"
					@submit.prevent="saveRole"
				>
					<input
						ref="roleInput"
						v-model="roleDraft"
						class="input is-small"
						maxlength="250"
						:placeholder="$t('people.rolePlaceholder')"
						:disabled="saving"
						@keydown.esc.stop.prevent="editingRole = false"
					>
					<XButton
						:loading="saving"
						:shadow="false"
						@click="saveRole"
					>
						{{ $t('misc.save') }}
					</XButton>
					<XButton
						variant="tertiary"
						:shadow="false"
						@click="editingRole = false"
					>
						{{ $t('misc.cancel') }}
					</XButton>
				</form>
				<p
					v-else
					class="person-role"
				>
					<span
						v-if="role"
						class="tag is-primary is-light"
					>{{ role }}</span>
					<span
						v-else
						class="has-text-grey is-italic"
					>{{ $t('people.noRole') }}</span>
					<BaseButton
						v-if="roleTeamId !== null"
						class="edit-role"
						@click="startEditRole"
					>
						<Icon icon="pen" />
						{{ $t('people.editRole') }}
					</BaseButton>
				</p>
			</div>
		</header>

		<div
			class="loader-container"
			:class="{'is-loading': loading}"
		>
			<section class="person-tasks">
				<h3>
					{{ $t('people.activeTasks') }}
					<span class="count">{{ active.length }}</span>
				</h3>
				<p
					v-if="!loading && active.length === 0"
					class="has-text-grey is-italic"
				>
					{{ $t('people.noActiveTasks') }}
				</p>
				<ul v-else>
					<li
						v-for="task in active"
						:key="task.id"
					>
						<RouterLink
							:to="{name: 'task.detail', params: {id: task.id}}"
							class="task-row"
							@click="$emit('navigate')"
						>
							<span class="task-title">{{ task.title }}</span>
							<span class="task-meta">
								<span
									v-if="projectTitle(task)"
									class="task-project"
								>{{ projectTitle(task) }}</span>
								<PriorityLabel
									:priority="task.priority"
									:done="task.done"
								/>
								<span
									v-if="task.dueDate"
									v-tooltip="formatDateLong(task.dueDate)"
									class="task-due"
									:class="{'is-overdue': isOverdue(task)}"
								>
									<Icon :icon="['far', 'calendar-alt']" />
									{{ formatDisplayDate(task.dueDate) }}
								</span>
							</span>
						</RouterLink>
					</li>
				</ul>
			</section>

			<section class="person-tasks">
				<h3>
					{{ $t('people.doneTasks') }}
					<span class="count">{{ done.length }}</span>
				</h3>
				<p
					v-if="!loading && done.length === 0"
					class="has-text-grey is-italic"
				>
					{{ $t('people.noDoneTasks') }}
				</p>
				<ul v-else>
					<li
						v-for="task in done"
						:key="task.id"
					>
						<RouterLink
							:to="{name: 'task.detail', params: {id: task.id}}"
							class="task-row is-done"
							@click="$emit('navigate')"
						>
							<span class="task-title">{{ task.title }}</span>
							<span class="task-meta">
								<span
									v-if="projectTitle(task)"
									class="task-project"
								>{{ projectTitle(task) }}</span>
								<span
									v-if="task.doneAt"
									v-tooltip="formatDateLong(task.doneAt)"
									class="task-done-at"
								>
									<Icon icon="check" />
									{{ formatDisplayDate(task.doneAt) }}
								</span>
							</span>
						</RouterLink>
					</li>
				</ul>
			</section>
		</div>
	</div>
</template>

<script setup lang="ts">
import {computed, nextTick, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'

import BaseButton from '@/components/base/BaseButton.vue'
import User from '@/components/misc/User.vue'
import PriorityLabel from '@/components/tasks/partials/PriorityLabel.vue'

import {findPerson, usePersonTasks} from '@/composables/usePeople'
import {getDisplayName} from '@/models/user'
import TeamMemberService from '@/services/teamMember'
import {useProjectStore} from '@/stores/projects'
import {formatDateLong, formatDisplayDate} from '@/helpers/time/formatDate'
import {success} from '@/message'

import type {ITask} from '@/modelTypes/ITask'
import type {IUser} from '@/modelTypes/IUser'

const props = withDefaults(defineProps<{
	user: IUser,
	compact?: boolean,
}>(), {
	compact: false,
})

defineEmits<{
	navigate: [],
}>()

const {t} = useI18n({useScope: 'global'})
const projectStore = useProjectStore()

const displayName = computed(() => getDisplayName(props.user))

const role = ref('')
const roleTeamId = ref<number | null>(null)
const {active, done, loading, load: loadTasks} = usePersonTasks()

const editingRole = ref(false)
const roleDraft = ref('')
const roleInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)

watch(() => props.user.username, async (username) => {
	role.value = ''
	roleTeamId.value = null
	editingRole.value = false
	const [person] = await Promise.all([
		findPerson(username),
		loadTasks(username),
	])
	role.value = person?.role ?? ''
	roleTeamId.value = person?.roleTeamId ?? null
}, {immediate: true})

function projectTitle(task: ITask) {
	return projectStore.projects[task.projectId]?.title ?? ''
}

function isOverdue(task: ITask) {
	return task.dueDate !== null && new Date(task.dueDate) < new Date()
}

async function startEditRole() {
	roleDraft.value = role.value
	editingRole.value = true
	await nextTick()
	roleInput.value?.focus()
}

async function saveRole() {
	if (roleTeamId.value === null || saving.value) {
		return
	}
	saving.value = true
	try {
		role.value = await new TeamMemberService().setRole(roleTeamId.value, props.user.username, roleDraft.value)
		editingRole.value = false
		success({message: t('people.roleSaved')})
	} finally {
		saving.value = false
	}
}
</script>

<style lang="scss" scoped>
.person-header {
	display: flex;
	align-items: center;
	gap: 1rem;
	margin-block-end: 1.5rem;
}

.person-heading {
	min-inline-size: 0;
	flex: 1;
}

.person-name {
	margin: 0;
	font-size: 1.4rem;
	line-height: 1.25;
	overflow-wrap: anywhere;
}

.person-username {
	margin: .1rem 0 .4rem;
	color: var(--grey-500);
	font-size: .85rem;
}

.person-role {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: .5rem;
	margin: 0;
}

.edit-role {
	color: var(--grey-500);
	font-size: .8rem;
	display: inline-flex;
	align-items: center;
	gap: .25rem;

	&:hover {
		color: var(--primary);
	}
}

.role-form {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .5rem;

	.input {
		flex: 1;
		min-inline-size: 10rem;
	}
}

.person-tasks {
	margin-block-end: 1.5rem;

	h3 {
		display: flex;
		align-items: center;
		gap: .5rem;
		font-size: 1rem;
		margin: 0 0 .5rem;
	}

	.count {
		font-size: .75rem;
		font-weight: normal;
		padding: .05rem .5rem;
		border-radius: 1rem;
		background: var(--grey-100);
		color: var(--grey-600);
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		border: 1px solid var(--grey-200);
		border-radius: $radius;
		overflow: hidden;
	}

	li + li {
		border-block-start: 1px solid var(--grey-200);
	}
}

.task-row {
	display: flex;
	flex-wrap: wrap;
	justify-content: space-between;
	align-items: center;
	gap: .25rem 1rem;
	padding: .5rem .75rem;
	color: var(--text);
	transition: background-color $transition;

	&:hover {
		background: var(--grey-100);
	}

	&.is-done .task-title {
		color: var(--grey-500);
	}
}

.task-title {
	overflow-wrap: anywhere;
}

.task-meta {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: .75rem;
	font-size: .8rem;
	color: var(--grey-500);

	:deep(.priority-label) {
		font-size: .8rem;
	}
}

.task-due.is-overdue {
	color: var(--danger);
	font-weight: bold;
}

.task-done-at {
	color: var(--success);
}
</style>
