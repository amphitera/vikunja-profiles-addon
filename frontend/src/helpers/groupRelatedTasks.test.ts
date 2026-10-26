import {describe, expect, it} from 'vitest'

import {groupRelatedTasks} from './groupRelatedTasks'
import type {ITask} from '@/modelTypes/ITask'

function task(id: number, related: Record<string, number[]> = {}): ITask {
	return {
		id,
		relatedTasks: Object.fromEntries(Object.entries(related).map(([kind, ids]) => [kind, ids.map(i => ({id: i}))])),
	} as unknown as ITask
}

describe('groupRelatedTasks', () => {
	it('keeps unrelated tasks in order', () => {
		const {tasks, positions} = groupRelatedTasks([task(1), task(2), task(3)])
		expect(tasks.map(t => t.id)).toEqual([1, 2, 3])
		expect(positions.size).toBe(0)
	})

	it('pulls related tasks to the first member, hub first', () => {
		const {tasks, positions} = groupRelatedTasks([
			task(1, {related: [5]}),
			task(2),
			task(3),
			task(5, {related: [1, 6]}),
			task(6, {related: [5]}),
		])
		expect(tasks.map(t => t.id)).toEqual([5, 1, 6, 2, 3])
		expect(positions.get(5)).toBe('first')
		expect(positions.get(1)).toBe('middle')
		expect(positions.get(6)).toBe('last')
		expect(positions.has(2)).toBe(false)
	})

	it('ignores subtask relations and tasks outside the view', () => {
		const {tasks, positions} = groupRelatedTasks([
			task(1, {subtask: [3], related: [99]}),
			task(2),
			task(3, {parenttask: [1]}),
		])
		expect(tasks.map(t => t.id)).toEqual([1, 2, 3])
		expect(positions.size).toBe(0)
	})
})
