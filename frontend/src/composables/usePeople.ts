import {computed, ref, shallowRef} from 'vue'

import TeamService from '@/services/team'
import TaskService from '@/services/task'
import {useAuthStore} from '@/stores/auth'
import {getDisplayName} from '@/models/user'

import type {IAbstract} from '@/modelTypes/IAbstract'
import type {ITask} from '@/modelTypes/ITask'
import type {ITeam} from '@/modelTypes/ITeam'
import type {ITeamMember} from '@/modelTypes/ITeamMember'
import type AbstractService from '@/services/abstractService'

export interface IPerson {
	user: ITeamMember
	role: string
	/** Team in which the viewer may change this person's role, null if they may not. */
	roleTeamId: number | null
}

const PER_PAGE = 50

async function fetchAllPages<T>(
	service: AbstractService,
	params: Record<string, unknown>,
): Promise<T[]> {
	const items: T[] = []
	let page = 1
	do {
		// getAll mutates params (page, snake_case), so every page gets a fresh copy
		const result = await service.getAll({} as IAbstract, {...params, per_page: PER_PAGE}, page)
		items.push(...(result as T[]))
		page++
	} while (page <= service.totalPages)
	return items
}

function collectPeople(teams: ITeam[], viewer: {id: number, isAdmin?: boolean} | null): IPerson[] {
	const people = new Map<number, IPerson>()
	const sortedTeams = [...teams].sort((a, b) => a.id - b.id)

	for (const team of sortedTeams) {
		const viewerIsTeamAdmin = viewer?.isAdmin === true
			|| team.members.some(m => m.id === viewer?.id && m.admin)

		for (const member of team.members) {
			let person = people.get(member.id)
			if (!person) {
				person = {user: member, role: '', roleTeamId: null}
				people.set(member.id, person)
			}
			if (person.role === '' && member.role) {
				person.role = member.role
				// The role is shown from this team, so edits must go to the same team.
				person.roleTeamId = viewerIsTeamAdmin ? team.id : null
			}
			if (person.role === '' && person.roleTeamId === null && viewerIsTeamAdmin) {
				person.roleTeamId = team.id
			}
		}
	}

	return [...people.values()].sort((a, b) =>
		getDisplayName(a.user).localeCompare(getDisplayName(b.user), undefined, {sensitivity: 'base'}),
	)
}

export function usePeople() {
	const authStore = useAuthStore()
	const people = ref<IPerson[]>([])
	const tasks = shallowRef<ITask[]>([])
	const loading = ref(false)

	async function load() {
		loading.value = true
		try {
			const [teams, allTasks] = await Promise.all([
				fetchAllPages<ITeam>(new TeamService(), {}),
				fetchAllPages<ITask>(new TaskService(), {
					sort_by: ['id'],
					order_by: ['desc'],
				}),
			])
			people.value = collectPeople(teams, authStore.info)
			tasks.value = allTasks
		} finally {
			loading.value = false
		}
	}

	const counts = computed(() => {
		const result = new Map<number, {active: number, done: number}>()
		for (const task of tasks.value) {
			for (const assignee of task.assignees ?? []) {
				const c = result.get(assignee.id) ?? {active: 0, done: 0}
				if (task.done) {
					c.done++
				} else {
					c.active++
				}
				result.set(assignee.id, c)
			}
		}
		return result
	})

	return {people, counts, loading, load}
}

export function usePersonTasks() {
	const active = ref<ITask[]>([])
	const done = ref<ITask[]>([])
	const loading = ref(false)

	async function load(username: string) {
		loading.value = true
		const assigneeFilter = `assignees in '${username.replace(/'/g, '')}'`
		try {
			const [activeTasks, doneTasks] = await Promise.all([
				fetchAllPages<ITask>(new TaskService(), {
					filter: `${assigneeFilter} && done = false`,
					sort_by: ['due_date', 'id'],
					order_by: ['asc', 'desc'],
				}),
				fetchAllPages<ITask>(new TaskService(), {
					filter: `${assigneeFilter} && done = true`,
					sort_by: ['done_at', 'id'],
					order_by: ['desc', 'desc'],
				}),
			])
			active.value = activeTasks
			done.value = doneTasks
		} finally {
			loading.value = false
		}
	}

	return {active, done, loading, load}
}

export async function findPerson(username: string): Promise<IPerson | null> {
	const authStore = useAuthStore()
	const teams = await fetchAllPages<ITeam>(new TeamService(), {})
	return collectPeople(teams, authStore.info).find(p => p.user.username === username) ?? null
}
