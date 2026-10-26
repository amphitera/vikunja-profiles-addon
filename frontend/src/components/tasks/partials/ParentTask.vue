<template>
	<div class="parent-task">
		<div
			v-if="parent && !editing"
			class="parent-task__current"
		>
			<RouterLink :to="{name: 'task.detail', params: {id: parent.id}}">
				{{ parent.title }}
			</RouterLink>
			<template v-if="canWrite">
				<BaseButton
					class="parent-task__action"
					@click="editing = true"
				>
					{{ $t('task.parent.change') }}
				</BaseButton>
				<BaseButton
					class="parent-task__action is-danger"
					@click="setParent(null)"
				>
					{{ $t('task.parent.remove') }}
				</BaseButton>
			</template>
		</div>
		<Multiselect
			v-else-if="canWrite"
			v-model="selected"
			:placeholder="$t('task.parent.searchPlaceholder')"
			:loading="taskService.loading"
			:search-results="foundTasks"
			label="title"
			@search="findTasks"
			@select="t => setParent(t as ITask)"
		/>
		<span
			v-else
			class="parent-task__none"
		>{{ $t('task.parent.none') }}</span>

		<div
			v-if="children.length > 0"
			class="parent-task__children"
		>
			<div class="parent-task__children-title">
				{{ $t('task.parent.children', {done: doneChildren, total: children.length}) }}
			</div>
			<ul>
				<li
					v-for="c in children"
					:key="c.id"
					:class="{'is-done': c.done}"
				>
					<RouterLink :to="{name: 'task.detail', params: {id: c.id}}">
						{{ c.title }}
					</RouterLink>
				</li>
			</ul>
		</div>
	</div>
</template>

<script setup lang="ts">
import {computed, ref, shallowReactive} from 'vue'

import BaseButton from '@/components/base/BaseButton.vue'
import Multiselect from '@/components/input/Multiselect.vue'
import TaskService from '@/services/task'
import TaskModel from '@/models/task'
import TaskRelationService from '@/services/taskRelation'
import TaskRelationModel from '@/models/taskRelation'
import {RELATION_KIND} from '@/types/IRelationKind'
import type {ITask} from '@/modelTypes/ITask'

const props = defineProps<{
	task: ITask,
	canWrite: boolean,
}>()

const emit = defineEmits<{
	'update:relatedTasks': [value: ITask['relatedTasks']],
}>()

const taskService = shallowReactive(new TaskService())
const relationService = shallowReactive(new TaskRelationService())

const editing = ref(false)
const selected = ref<ITask | null>(null)
const foundTasks = ref<ITask[]>([])

const parent = computed(() => props.task.relatedTasks?.parenttask?.[0] ?? null)
const children = computed(() => props.task.relatedTasks?.subtask ?? [])
const doneChildren = computed(() => children.value.filter(c => c.done).length)

async function findTasks(query: string) {
	const result = await taskService.getAll(new TaskModel(), {s: query, sort_by: 'done'})
	foundTasks.value = result.filter(t => t.id !== props.task.id && !children.value.some(c => c.id === t.id))
}

async function setParent(newParent: ITask | null) {
	const related = {...props.task.relatedTasks}
	if (parent.value) {
		await relationService.delete(new TaskRelationModel({
			taskId: props.task.id,
			otherTaskId: parent.value.id,
			relationKind: RELATION_KIND.PARENTTASK,
		}))
		related.parenttask = []
	}
	if (newParent) {
		await relationService.create(new TaskRelationModel({
			taskId: props.task.id,
			otherTaskId: newParent.id,
			relationKind: RELATION_KIND.PARENTTASK,
		}))
		related.parenttask = [newParent]
	}
	editing.value = false
	selected.value = null
	emit('update:relatedTasks', related)
}
</script>

<style lang="scss" scoped>
.parent-task__current {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: .75rem;
}

.parent-task__action {
	font-size: .8rem;
	color: var(--grey-500);

	&:hover {
		color: var(--primary);
	}

	&.is-danger:hover {
		color: var(--danger);
	}
}

.parent-task__none {
	color: var(--grey-500);
	font-style: italic;
}

.parent-task__children {
	margin-block-start: .5rem;

	ul {
		margin: .25rem 0 0;
		padding-inline-start: 1rem;
	}

	li.is-done a {
		text-decoration: line-through;
		color: var(--grey-500);
	}
}

.parent-task__children-title {
	font-size: .85rem;
	color: var(--grey-500);
}
</style>
