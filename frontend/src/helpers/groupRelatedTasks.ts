import type {ITask} from '@/modelTypes/ITask'

export type TaskGroupPosition = 'first' | 'middle' | 'last'

// Subtasks are already rendered nested under their parent, so only the other kinds pull tasks together.
const NESTED_KINDS = new Set(['subtask', 'parenttask'])

/**
 * Reorders tasks so that tasks linked by a relation stand next to each other.
 * A group takes the place of its first member in the incoming order, starts with
 * its most-linked task and otherwise keeps the incoming order.
 */
export function groupRelatedTasks(tasks: ITask[]): {tasks: ITask[], positions: Map<ITask['id'], TaskGroupPosition>} {
	const inView = new Map(tasks.map(t => [t.id, t]))
	const links = new Map<ITask['id'], Set<ITask['id']>>()
	const link = (a: number, b: number) => {
		if (!links.has(a)) links.set(a, new Set())
		links.get(a)!.add(b)
	}
	for (const t of tasks) {
		for (const [kind, related] of Object.entries(t.relatedTasks ?? {})) {
			if (NESTED_KINDS.has(kind)) {
				continue
			}
			for (const other of related ?? []) {
				if (other.id !== t.id && inView.has(other.id)) {
					link(t.id, other.id)
					link(other.id, t.id)
				}
			}
		}
	}

	const order = new Map(tasks.map((t, i) => [t.id, i]))
	const placed = new Set<ITask['id']>()
	const result: ITask[] = []
	const positions = new Map<ITask['id'], TaskGroupPosition>()

	for (const t of tasks) {
		if (placed.has(t.id)) {
			continue
		}
		const group: ITask['id'][] = []
		const queue = [t.id]
		placed.add(t.id)
		while (queue.length > 0) {
			const id = queue.shift()!
			group.push(id)
			for (const next of links.get(id) ?? []) {
				if (!placed.has(next)) {
					placed.add(next)
					queue.push(next)
				}
			}
		}
		group.sort((a, b) =>
			(links.get(b)?.size ?? 0) - (links.get(a)?.size ?? 0) || order.get(a)! - order.get(b)!,
		)
		for (const [i, id] of group.entries()) {
			result.push(inView.get(id)!)
			if (group.length > 1) {
				positions.set(id, i === 0 ? 'first' : i === group.length - 1 ? 'last' : 'middle')
			}
		}
	}
	return {tasks: result, positions}
}
