<template>
	<div
		class="content loader-container is-max-width-desktop"
		:class="{'is-loading': loading}"
	>
		<RouterLink
			:to="{name: 'people.index'}"
			class="back-link"
		>
			<Icon icon="arrow-left" />
			{{ $t('people.title') }}
		</RouterLink>
		<Card v-if="user">
			<PersonProfile :user="user" />
		</Card>
		<p
			v-else-if="!loading"
			class="has-text-centered has-text-grey is-italic"
		>
			{{ $t('people.notFound') }}
		</p>
	</div>
</template>

<script setup lang="ts">
import {ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'

import Card from '@/components/misc/Card.vue'
import PersonProfile from '@/components/people/PersonProfile.vue'
import {findPerson} from '@/composables/usePeople'
import {useTitle} from '@/composables/useTitle'
import {getDisplayName} from '@/models/user'

import type {IUser} from '@/modelTypes/IUser'

const props = defineProps<{
	username: string,
}>()

const {t} = useI18n({useScope: 'global'})

const user = ref<IUser | null>(null)
const loading = ref(false)

useTitle(() => user.value ? getDisplayName(user.value) : t('people.title'))

watch(() => props.username, async (username) => {
	loading.value = true
	try {
		user.value = (await findPerson(username))?.user ?? null
	} finally {
		loading.value = false
	}
}, {immediate: true})
</script>

<style lang="scss" scoped>
.back-link {
	display: inline-flex;
	align-items: center;
	gap: .4rem;
	margin-block-end: 1rem;
	color: var(--grey-500);
}
</style>
