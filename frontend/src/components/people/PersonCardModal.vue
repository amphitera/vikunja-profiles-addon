<template>
	<Modal
		:enabled="personCard.user !== null"
		class="person-card-modal"
		:aria-label="$t('people.profile')"
		@close="personCard.close()"
	>
		<Card
			v-if="personCard.user"
			class="person-card"
			:shadow="false"
		>
			<PersonProfile
				:user="personCard.user"
				compact
				@navigate="personCard.close()"
			/>
			<template #footer>
				<RouterLink
					:to="{name: 'people.show', params: {username: personCard.user.username}}"
					class="card-footer-item"
					@click="personCard.close()"
				>
					{{ $t('people.openProfile') }}
				</RouterLink>
			</template>
		</Card>
	</Modal>
</template>

<script setup lang="ts">
import {watch} from 'vue'
import {useRoute} from 'vue-router'

import Modal from '@/components/misc/Modal.vue'
import Card from '@/components/misc/Card.vue'
import PersonProfile from '@/components/people/PersonProfile.vue'
import {usePersonCardStore} from '@/stores/personCard'

const personCard = usePersonCardStore()
const route = useRoute()

watch(() => route.fullPath, () => personCard.close())
</script>

<style lang="scss" scoped>
.person-card {
	inline-size: min(40rem, calc(100vw - 2rem));
	max-block-size: calc(100vh - 4rem);
	overflow-y: auto;
	margin: 0 auto;
	// Modal centers its content text by default.
	text-align: start;
}
</style>
