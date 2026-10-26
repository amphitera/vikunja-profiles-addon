<template>
	<div
		v-if="lines.length > 0"
		class="task-history content details"
	>
		<BaseButton
			class="task-history__toggle task-section-title"
			@click="open = !open"
		>
			<span class="icon is-grey">
				<Icon icon="history" />
			</span>
			{{ $t('task.history.title') }} ({{ lines.length }})
			<Icon
				icon="chevron-down"
				class="task-history__chevron"
				:class="{'is-open': open}"
			/>
		</BaseButton>
		<ul
			v-if="open"
			class="task-history__list"
		>
			<li
				v-for="line in lines"
				:key="line.key"
			>
				<span class="task-history__when">{{ line.when }}</span>
				<span
					v-if="line.who"
					class="task-history__who"
				>{{ line.who }}</span>
				<span>{{ line.text }}</span>
			</li>
		</ul>
	</div>
</template>

<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'

import BaseButton from '@/components/base/BaseButton.vue'
import {AuthenticatedHTTPFactory, apiV2Url} from '@/helpers/fetcher'
import {formatDay} from '@/helpers/taskPeriod'
import type {ITask} from '@/modelTypes/ITask'

interface RawTaskEvent {
	id: number
	kind: TaskEvent['kind']
	old_value: string | null
	new_value: string | null
	created: string
	user: {name?: string, username?: string} | null
}

interface TaskEvent {
	id: number
	kind: 'assignee_added' | 'assignee_removed' | 'end_date_changed'
	oldValue: string
	newValue: string
	created: string
	who: string
}

const props = defineProps<{
	task: ITask,
	// Bumped by the parent after every save so the history picks up the new events.
	refreshKey: number,
}>()

const {t, locale} = useI18n({useScope: 'global'})
const open = ref(false)
const events = ref<TaskEvent[]>([])

// Assignments made together with creating the task are part of the setup, not history.
const SETUP_WINDOW_MS = 2 * 60 * 1000
// A removal and an addition by the same person this close together read as one reassignment.
const REASSIGN_WINDOW_MS = 2 * 60 * 1000

async function load() {
	if (!props.task.id) {
		return
	}
	const http = AuthenticatedHTTPFactory()
	const {data} = await http.get(apiV2Url(`tasks/${props.task.id}/events?per_page=500`))
	events.value = ((data.items ?? []) as RawTaskEvent[]).map((e): TaskEvent => ({
		id: e.id,
		kind: e.kind,
		oldValue: e.old_value ?? '',
		newValue: e.new_value ?? '',
		created: e.created,
		who: e.user?.name || e.user?.username || '',
	}))
}

watch(() => [props.task.id, props.refreshKey], load, {immediate: true})

const lines = computed(() => {
	const created = new Date(props.task.created).getTime()
	const relevant = events.value.filter(e => !(
		e.kind === 'assignee_added' && new Date(e.created).getTime() - created < SETUP_WINDOW_MS
	))

	const result: {key: string, when: string, who: string, text: string}[] = []
	const used = new Set<number>()
	for (const [i, e] of relevant.entries()) {
		if (used.has(e.id)) {
			continue
		}
		const when = new Date(e.created).toLocaleString(locale.value, {day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit'})
		let text: string
		if (e.kind === 'end_date_changed') {
			if (!e.oldValue) {
				text = t('task.history.endSet', {to: formatDay(e.newValue, locale.value)})
			} else if (!e.newValue) {
				text = t('task.history.endRemoved')
			} else {
				text = t('task.history.endMoved', {from: formatDay(e.oldValue, locale.value), to: formatDay(e.newValue, locale.value)})
			}
		} else {
			const partner = relevant.slice(i + 1).find(o =>
				!used.has(o.id) &&
				o.who === e.who &&
				o.kind !== e.kind && o.kind !== 'end_date_changed' &&
				Math.abs(new Date(o.created).getTime() - new Date(e.created).getTime()) < REASSIGN_WINDOW_MS,
			)
			if (partner) {
				used.add(partner.id)
				const removed = e.kind === 'assignee_removed' ? e.oldValue : partner.oldValue
				const added = e.kind === 'assignee_added' ? e.newValue : partner.newValue
				text = t('task.history.reassigned', {from: removed, to: added})
			} else if (e.kind === 'assignee_added') {
				text = t('task.history.assigned', {name: e.newValue})
			} else {
				text = t('task.history.unassigned', {name: e.oldValue})
			}
		}
		result.push({key: String(e.id), when, who: e.who, text})
	}
	return result
})
</script>

<style lang="scss" scoped>
.task-history__toggle {
	display: flex;
	align-items: center;
	gap: .25rem;
}

.task-history__chevron {
	margin-inline-start: .25rem;
	transition: transform $transition;

	&.is-open {
		transform: rotate(180deg);
	}
}

.task-history__list {
	list-style: none;
	margin: .5rem 0 0;
	padding: 0;
	color: var(--grey-500);
	font-size: .875rem;

	li {
		display: flex;
		flex-wrap: wrap;
		gap: .5rem;
		padding: .25rem 0;
		margin: 0;
	}
}

.task-history__when {
	min-inline-size: 8.5rem;
}

.task-history__who {
	color: var(--grey-700);
}
</style>
