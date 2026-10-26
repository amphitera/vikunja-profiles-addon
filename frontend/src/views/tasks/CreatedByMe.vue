<template>
	<div
		class="content loader-container is-max-width-desktop created-by-me"
		:class="{'is-loading': loading}"
	>
		<div class="created-by-me__header">
			<h1>{{ $t('createdByMe.title') }}</h1>
			<div class="created-by-me__controls">
				<label class="created-by-me__sort">
					<span>{{ $t('createdByMe.sort') }}</span>
					<div class="select">
						<select v-model="sortBy">
							<option value="end">{{ $t('createdByMe.sortEnd') }}</option>
							<option value="start">{{ $t('createdByMe.sortStart') }}</option>
							<option value="assignee">{{ $t('createdByMe.sortAssignee') }}</option>
							<option value="title">{{ $t('createdByMe.sortTitle') }}</option>
							<option value="created">{{ $t('createdByMe.sortCreated') }}</option>
						</select>
					</div>
				</label>
				<FancyCheckbox v-model="showDone">
					{{ $t('createdByMe.showDone') }}
				</FancyCheckbox>
			</div>
		</div>

		<Card
			v-if="sorted.length > 0"
			:padding="false"
			:has-content="false"
		>
			<ul class="created-by-me__list">
				<li
					v-for="task in sorted"
					:key="task.id"
					:class="{'is-done': task.done}"
				>
					<BaseButton
						v-tooltip="task.done ? $t('createdByMe.markUndone') : $t('createdByMe.markDone')"
						class="created-by-me__check"
						:aria-label="task.done ? $t('createdByMe.markUndone') : $t('createdByMe.markDone')"
						@click="toggleDone(task)"
					>
						<Icon :icon="task.done ? ['far', 'square-check'] : ['far', 'square']" />
					</BaseButton>
					<div class="created-by-me__main">
						<RouterLink
							:to="{name: 'task.detail', params: {id: task.id}}"
							class="created-by-me__title"
						>
							{{ task.title }}
						</RouterLink>
						<span
							v-if="task.relatedTasks?.parenttask?.[0]"
							class="created-by-me__parent"
						>{{ task.relatedTasks.parenttask[0].title }}</span>
					</div>
					<div class="created-by-me__assignees">
						<template v-if="task.assignees.length > 0">
							{{ task.assignees.map(getDisplayName).join(', ') }}
						</template>
						<span
							v-else
							class="is-muted"
						>{{ $t('createdByMe.nobody') }}</span>
					</div>
					<div
						class="created-by-me__period"
						:class="{'is-overdue': isOverdue(task)}"
					>
						<template v-if="task.endDate">
							<span v-if="task.startDate">{{ formatDay(task.startDate, locale) }} — </span>{{ formatDay(task.endDate, locale) }}
						</template>
						<span
							v-else
							class="is-muted"
						>{{ $t('createdByMe.noDeadline') }}</span>
					</div>
				</li>
			</ul>
		</Card>
		<p
			v-else-if="!loading"
			class="has-text-grey"
		>
			{{ $t('createdByMe.empty') }}
		</p>
	</div>
</template>

<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useStorage} from '@vueuse/core'

import BaseButton from '@/components/base/BaseButton.vue'
import Card from '@/components/misc/Card.vue'
import FancyCheckbox from '@/components/input/FancyCheckbox.vue'
import {useCreatedByMe} from '@/composables/useCreatedByMe'
import {useTitle} from '@/composables/useTitle'
import {daysUntil, formatDay} from '@/helpers/taskPeriod'
import {getDisplayName} from '@/models/user'
import {useTaskStore} from '@/stores/tasks'
import type {ITask} from '@/modelTypes/ITask'

type SortKey = 'end' | 'start' | 'assignee' | 'title' | 'created'

const {t, locale} = useI18n({useScope: 'global'})
useTitle(() => t('createdByMe.title'))

const {tasks, loading, load} = useCreatedByMe()
const taskStore = useTaskStore()

const sortBy = useStorage<SortKey>('createdByMeSort', 'end')
const showDone = ref(false)

watch(showDone, v => load(v), {immediate: true})

const time = (d: Date | null) => d ? new Date(d).getTime() : Infinity

const sorted = computed(() => {
	const list = tasks.value.filter(t => showDone.value || !t.done)
	const byTitle = (a: ITask, b: ITask) => a.title.localeCompare(b.title, locale.value)
	const compare: Record<SortKey, (a: ITask, b: ITask) => number> = {
		end: (a, b) => time(a.endDate) - time(b.endDate) || byTitle(a, b),
		start: (a, b) => time(a.startDate) - time(b.startDate) || byTitle(a, b),
		assignee: (a, b) => {
			// Tasks without an assignee go last.
			const name = (x: ITask) => x.assignees[0] ? getDisplayName(x.assignees[0]) : null
			const na = name(a)
			const nb = name(b)
			if (na === null || nb === null) {
				return Number(na === null) - Number(nb === null) || time(a.endDate) - time(b.endDate)
			}
			return na.localeCompare(nb, locale.value) || time(a.endDate) - time(b.endDate)
		},
		title: byTitle,
		created: (a, b) => time(b.created) - time(a.created),
	}
	return [...list].sort(compare[sortBy.value] ?? compare.end)
})

function isOverdue(task: ITask) {
	const days = daysUntil(task.endDate)
	return !task.done && days !== null && days < 0
}

async function toggleDone(task: ITask) {
	const updated = await taskStore.update({...task, done: !task.done})
	Object.assign(task, updated)
}
</script>

<style lang="scss" scoped>
.created-by-me__header {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	margin-block-end: 1rem;

	h1 {
		margin: 0;
	}
}

.created-by-me__controls {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 1rem;
}

.created-by-me__sort {
	display: flex;
	align-items: center;
	gap: .5rem;
	color: var(--grey-500);
}

.created-by-me__list {
	list-style: none;
	margin: 0;
	padding: 0;

	li {
		display: grid;
		grid-template-columns: 2rem minmax(0, 1fr) minmax(8rem, 14rem) 9rem;
		align-items: center;
		gap: .75rem;
		padding: .6rem 1rem;
		margin: 0;
		border-block-end: 1px solid var(--grey-200);

		&:last-child {
			border-block-end: none;
		}

		&.is-done .created-by-me__title {
			text-decoration: line-through;
			color: var(--grey-500);
		}

		@media screen and (max-width: $tablet) {
			grid-template-columns: 2rem minmax(0, 1fr);

			.created-by-me__assignees,
			.created-by-me__period {
				grid-column: 2;
			}
		}
	}
}

.created-by-me__check {
	color: var(--grey-500);

	&:hover {
		color: var(--success);
	}
}

.created-by-me__main {
	display: flex;
	flex-direction: column;
	min-inline-size: 0;
}

.created-by-me__parent {
	font-size: .8rem;
	color: var(--grey-500);
}

.created-by-me__assignees {
	font-size: .9rem;
	color: var(--grey-700);
}

.created-by-me__period {
	font-size: .9rem;
	text-align: end;

	&.is-overdue {
		color: var(--danger);
		font-weight: 600;
	}

	@media screen and (max-width: $tablet) {
		text-align: start;
	}
}

.is-muted {
	color: var(--grey-500);
	font-style: italic;
}
</style>
