import {computed, ref} from 'vue'

import TaskService from '@/services/task'
import TaskModel from '@/models/task'
import {useAuthStore} from '@/stores/auth'
import type {ITask} from '@/modelTypes/ITask'

// Module-level so the menu counter and the page share one list.
const tasks = ref<ITask[]>([])
const loading = ref(false)

const PER_PAGE = 50

export function useCreatedByMe() {
	const authStore = useAuthStore()

	async function load(includeDone = false) {
		const username = authStore.info?.username
		if (!username) {
			return
		}
		loading.value = true
		try {
			const service = new TaskService()
			const filter = `created_by = '${username.replace(/'/g, '')}'` + (includeDone ? '' : ' && done = false')
			const all: ITask[] = []
			for (let page = 1; ; page++) {
				const batch = await service.getAll(new TaskModel(), {filter, per_page: PER_PAGE}, page)
				all.push(...batch)
				if (batch.length < PER_PAGE) {
					break
				}
			}
			tasks.value = all
		} finally {
			loading.value = false
		}
	}

	const openCount = computed(() => tasks.value.filter(t => !t.done).length)

	return {tasks, loading, load, openCount}
}
