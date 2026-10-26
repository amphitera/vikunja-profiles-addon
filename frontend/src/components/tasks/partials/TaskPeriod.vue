<template>
	<div class="task-period">
		<div class="task-period__dates">
			<span class="task-period__word">{{ $t('task.period.from') }}</span>
			<Datepicker
				:model-value="startDate"
				:choose-date-label="$t('task.period.chooseStart')"
				:disabled="disabled"
				@update:modelValue="d => emitChange({startDate: d as Date | null, endDate})"
				@closeOnChange="emit('save')"
			/>
			<span class="task-period__word">{{ $t('task.period.to') }}</span>
			<Datepicker
				:model-value="endDate"
				:choose-date-label="$t('task.period.chooseEnd')"
				:disabled="disabled"
				@update:modelValue="d => emitChange({startDate, endDate: d as Date | null})"
				@closeOnChange="emit('save')"
			/>
		</div>
		<div
			v-if="!disabled"
			class="task-period__presets"
		>
			<BaseButton
				v-for="weeks in [1, 2]"
				:key="weeks"
				class="task-period__preset"
				@click="applyPreset(weeks)"
			>
				{{ $t('task.period.weeks', {count: weeks}, weeks) }}
			</BaseButton>
			<BaseButton
				v-if="startDate || endDate"
				class="task-period__preset"
				@click="clear"
			>
				{{ $t('task.period.none') }}
			</BaseButton>
		</div>
		<p
			v-if="deadlineHint"
			class="task-period__hint"
			:class="{'is-overdue': isOverdue}"
		>
			{{ deadlineHint }}
		</p>
	</div>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'

import BaseButton from '@/components/base/BaseButton.vue'
import Datepicker from '@/components/input/Datepicker.vue'
import {daysUntil, periodFromPreset} from '@/helpers/taskPeriod'

const props = defineProps<{
	startDate: Date | null,
	endDate: Date | null,
	done?: boolean,
	disabled?: boolean,
}>()

const emit = defineEmits<{
	'update': [value: {startDate: Date | null, endDate: Date | null}],
	// Presets and "no deadline" change both dates at once and save right away.
	'save': [],
}>()

const {t} = useI18n({useScope: 'global'})

function emitChange(value: {startDate: Date | null, endDate: Date | null}) {
	emit('update', value)
}

function applyPreset(weeks: number) {
	emit('update', periodFromPreset(props.startDate, weeks))
	emit('save')
}

function clear() {
	emit('update', {startDate: null, endDate: null})
	emit('save')
}

const days = computed(() => daysUntil(props.endDate))
const isOverdue = computed(() => !props.done && days.value !== null && days.value < 0)

const deadlineHint = computed(() => {
	if (props.done || days.value === null) {
		return ''
	}
	if (days.value < 0) {
		return t('task.period.overdue', {count: -days.value}, -days.value)
	}
	if (days.value === 0) {
		return t('task.period.today')
	}
	return t('task.period.left', {count: days.value}, days.value)
})
</script>

<style lang="scss" scoped>
.task-period__dates {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: .25rem;

	:deep(.datepicker) {
		inline-size: auto;
	}
}

.task-period__word {
	color: var(--grey-500);
}

.task-period__presets {
	display: flex;
	flex-wrap: wrap;
	gap: .5rem;
	margin-block-start: .25rem;
}

.task-period__preset {
	font-size: .8rem;
	padding: .1rem .6rem;
	border: 1px solid var(--grey-300);
	border-radius: 1rem;
	color: var(--grey-700);

	&:hover {
		border-color: var(--primary);
		color: var(--primary);
	}
}

.task-period__hint {
	margin: .25rem 0 0;
	font-size: .85rem;
	color: var(--grey-500);

	&.is-overdue {
		color: var(--danger);
		font-weight: 600;
	}
}
</style>
