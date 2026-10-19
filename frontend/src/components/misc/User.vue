<template>
	<div
		class="user"
		:class="{'is-inline': isInline}"
		:style="{'--avatar-size': `${avatarSize}px`}"
	>
		<component
			:is="clickable ? 'button' : 'span'"
			class="avatar-wrapper"
			:class="{'is-clickable': clickable}"
			:type="clickable ? 'button' : undefined"
			:aria-label="clickable ? t('people.showProfileOf', {user: displayName}) : undefined"
			@click="clickable && openPersonCard($event)"
		>
			<img
				v-tooltip="displayName"
				:height="avatarSize"
				:src="avatarSrc"
				:width="avatarSize"
				:alt="'Avatar of ' + displayName"
				class="avatar"
			>
			<span
				v-if="isBot"
				v-tooltip="t('user.settings.bots.badge')"
				class="bot-badge"
				aria-label="Bot"
			>B</span>
		</component>
		<span
			v-if="showUsername"
			class="username"
		>{{ displayName }}</span>
	</div>
</template>

<script lang="ts" setup>
import {computed, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'

import {fetchAvatarBlobUrl, getDisplayName} from '@/models/user'
import type {IUser} from '@/modelTypes/IUser'
import {usePersonCardStore} from '@/stores/personCard'

const props = withDefaults(defineProps<{
	user: IUser,
	showUsername?: boolean,
	avatarSize?: number,
	isInline?: boolean,
	clickable?: boolean,
}>(), {
	showUsername: true,
	avatarSize: 50,
	isInline: false,
	clickable: false,
})

const {t} = useI18n({useScope: 'global'})

const displayName = computed(() => getDisplayName(props.user))
const isBot = computed(() => ((props.user as IUser & {botOwnerId?: number}).botOwnerId ?? 0) > 0)
const avatarSrc = ref('')

const personCard = usePersonCardStore()

// Avatars sit inside task rows and kanban cards, which navigate on click themselves.
function openPersonCard(event: MouseEvent) {
	event.preventDefault()
	event.stopPropagation()
	personCard.open(props.user)
}

async function loadAvatar() {
	avatarSrc.value = await fetchAvatarBlobUrl(props.user, props.avatarSize)
}

watch(() => [props.user, props.avatarSize], loadAvatar, { immediate: true })
</script>

<style lang="scss" scoped>
.user {
	display: flex;
	justify-items: center;

	&.is-inline {
		display: inline-flex;
	}
}

.avatar-wrapper {
	position: relative;
	display: inline-flex;
	margin-inline-end: .5rem;

	&.is-clickable {
		padding: 0;
		border: 0;
		background: none;
		border-radius: 100%;
		cursor: pointer;
		transition: transform $transition;

		&:hover,
		&:focus-visible {
			transform: scale(1.1);
		}
	}
}

.avatar {
	inline-size: var(--avatar-size);
	block-size: var(--avatar-size);
	border-radius: 100%;
	vertical-align: middle;
}

.bot-badge {
	position: absolute;
	inset-block-end: 0;
	inset-inline-start: 0;
	display: inline-flex;
	align-items: center;
	justify-content: center;
	inline-size: 40%;
	block-size: 40%;
	min-inline-size: 14px;
	min-block-size: 14px;
	max-inline-size: 22px;
	max-block-size: 22px;
	font-size: .65rem;
	font-weight: 700;
	line-height: 1;
	color: var(--white);
	background: var(--primary);
	border: 2px solid var(--white);
	border-radius: 100%;
	text-transform: uppercase;
	pointer-events: auto;
}
</style>
