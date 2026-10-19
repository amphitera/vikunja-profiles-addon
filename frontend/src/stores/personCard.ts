import {ref} from 'vue'
import {acceptHMRUpdate, defineStore} from 'pinia'

import type {IUser} from '@/modelTypes/IUser'

export const usePersonCardStore = defineStore('personCard', () => {
	const user = ref<IUser | null>(null)

	function open(person: IUser) {
		user.value = person
	}

	function close() {
		user.value = null
	}

	return {
		user,
		open,
		close,
	}
})

// support hot reloading
if (import.meta.hot) {
	import.meta.hot.accept(acceptHMRUpdate(usePersonCardStore, import.meta.hot))
}
