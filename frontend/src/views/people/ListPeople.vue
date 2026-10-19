<template>
	<div
		class="content loader-container is-max-width-desktop"
		:class="{'is-loading': loading}"
	>
		<h1>{{ $t('people.title') }}</h1>
		<Card
			v-if="people.length > 0"
			:padding="false"
			:has-content="false"
		>
			<ul class="people">
				<li
					v-for="person in people"
					:key="person.user.id"
				>
					<RouterLink
						:to="{name: 'people.show', params: {username: person.user.username}}"
						class="person-row"
					>
						<User
							:user="person.user"
							:avatar-size="40"
							:show-username="false"
						/>
						<span class="person-info">
							<span class="person-name">{{ getDisplayName(person.user) }}</span>
							<span
								v-if="person.role"
								class="person-role"
							>{{ person.role }}</span>
						</span>
						<span class="person-counts">
							<span
								v-tooltip="$t('people.activeTasks')"
								class="count is-active"
							>
								<Icon :icon="['far', 'clock']" />
								{{ counts.get(person.user.id)?.active ?? 0 }}
							</span>
							<span
								v-tooltip="$t('people.doneTasks')"
								class="count is-done"
							>
								<Icon icon="check" />
								{{ counts.get(person.user.id)?.done ?? 0 }}
							</span>
						</span>
					</RouterLink>
				</li>
			</ul>
		</Card>
		<p
			v-else-if="!loading"
			class="has-text-centered has-text-grey is-italic"
		>
			{{ $t('people.nobody') }}
		</p>
	</div>
</template>

<script setup lang="ts">
import {useI18n} from 'vue-i18n'

import Card from '@/components/misc/Card.vue'
import User from '@/components/misc/User.vue'
import {usePeople} from '@/composables/usePeople'
import {useTitle} from '@/composables/useTitle'
import {getDisplayName} from '@/models/user'

const {t} = useI18n({useScope: 'global'})
useTitle(() => t('people.title'))

const {people, counts, loading, load} = usePeople()
load()
</script>

<style lang="scss" scoped>
ul.people {
	padding: 0;
	margin: 0;
	border-radius: $radius;
	overflow: hidden;

	li {
		list-style: none;
		margin: 0;
	}

	li + li {
		border-block-start: 1px solid var(--grey-200);
	}
}

.person-row {
	display: flex;
	align-items: center;
	gap: .75rem;
	padding: .6rem 1rem;
	color: var(--text);
	transition: background-color $transition;

	&:hover {
		background: var(--grey-100);
	}

	:deep(.avatar-wrapper) {
		margin-inline-end: 0;
	}
}

.person-info {
	display: flex;
	flex-direction: column;
	flex: 1;
	min-inline-size: 0;
}

.person-name {
	overflow-wrap: anywhere;
}

.person-role {
	font-size: .8rem;
	color: var(--grey-500);
}

.person-counts {
	display: flex;
	gap: .75rem;
	font-size: .85rem;
	white-space: nowrap;
}

.count {
	display: inline-flex;
	align-items: center;
	gap: .3rem;

	&.is-active {
		color: var(--primary);
	}

	&.is-done {
		color: var(--success);
	}
}
</style>
