<template>
	<Modal
		:enabled="enabled"
		:overflow="true"
		@close="emit('close')"
	>
		<Card
			:title="$t('drive.pickTitle')"
			:shadow="false"
			:padding="false"
			class="has-text-start"
			:loading="loading"
			:show-close="true"
			@close="emit('close')"
		>
			<div class="p-4 drive-picker">
				<nav class="drive-picker__crumbs">
					<BaseButton @click="open(0)">
						{{ $t('drive.title') }}
					</BaseButton>
					<template
						v-for="f in listing?.path ?? []"
						:key="f.id"
					>
						<span>›</span>
						<BaseButton @click="open(f.id)">
							{{ f.name }}
						</BaseButton>
					</template>
				</nav>
				<ul>
					<li
						v-for="f in listing?.folders ?? []"
						:key="'f' + f.id"
					>
						<BaseButton @click="open(f.id)">
							<Icon
								icon="folder"
								class="drive-picker__icon is-folder"
							/>
							{{ f.name }}
						</BaseButton>
					</li>
					<li
						v-for="i in listing?.items ?? []"
						:key="'i' + i.id"
					>
						<BaseButton @click="emit('pick', i)">
							<Icon
								icon="file"
								class="drive-picker__icon"
							/>
							{{ i.name }}
							<span class="drive-picker__size">{{ formatSize(i.size) }}</span>
						</BaseButton>
					</li>
					<li
						v-if="listing && listing.folders.length === 0 && listing.items.length === 0"
						class="drive-picker__empty"
					>
						{{ $t('drive.empty') }}
					</li>
				</ul>
			</div>
		</Card>
	</Modal>
</template>

<script setup lang="ts">
import {ref, watch} from 'vue'

import BaseButton from '@/components/base/BaseButton.vue'
import Card from '@/components/misc/Card.vue'
import Modal from '@/components/misc/Modal.vue'
import {formatSize, getListing, type DriveItem, type DriveListing} from '@/services/drive'

const props = defineProps<{
	enabled: boolean,
	projectId: number,
}>()

const emit = defineEmits<{
	'close': [],
	'pick': [item: DriveItem],
}>()

const loading = ref(false)
const listing = ref<DriveListing | null>(null)

async function open(folderId: number) {
	loading.value = true
	try {
		listing.value = await getListing(props.projectId, folderId)
	} finally {
		loading.value = false
	}
}

watch(() => props.enabled, enabled => {
	if (enabled) {
		open(0)
	}
})
</script>

<style lang="scss" scoped>
.drive-picker__crumbs {
	display: flex;
	flex-wrap: wrap;
	gap: .35rem;
	margin-block-end: .75rem;
	color: var(--grey-500);
}

.drive-picker ul {
	list-style: none;
	margin: 0;
	padding: 0;
	max-block-size: 50vh;
	overflow: auto;

	li {
		margin: 0;
	}

	.base-button {
		display: flex;
		align-items: center;
		inline-size: 100%;
		padding: .4rem .25rem;
		text-align: start;

		&:hover {
			background: var(--grey-100);
		}
	}
}

.drive-picker__icon {
	margin-inline-end: .5rem;
	color: var(--grey-500);

	&.is-folder {
		color: var(--primary);
	}
}

.drive-picker__size {
	margin-inline-start: auto;
	color: var(--grey-500);
	font-size: .85rem;
}

.drive-picker__empty {
	color: var(--grey-500);
	padding: 1rem 0;
}
</style>
