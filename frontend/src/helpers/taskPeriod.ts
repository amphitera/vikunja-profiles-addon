// End of the working day: deadlines set through the period buttons expire then, not at midnight.
const END_OF_DAY_HOUR = 18

export function periodFromPreset(start: Date | null, weeks: number): {startDate: Date, endDate: Date} {
	const startDate = start ? new Date(start) : new Date()
	if (!start) {
		startDate.setHours(9, 0, 0, 0)
	}
	const endDate = new Date(startDate)
	endDate.setDate(endDate.getDate() + weeks * 7)
	endDate.setHours(END_OF_DAY_HOUR, 0, 0, 0)
	return {startDate, endDate}
}

export function formatDay(date: Date | string | null, locale: string): string {
	if (!date) {
		return ''
	}
	const d = new Date(date)
	const sameYear = d.getFullYear() === new Date().getFullYear()
	return d.toLocaleDateString(locale, sameYear
		? {day: 'numeric', month: 'short'}
		: {day: 'numeric', month: 'short', year: 'numeric'})
}

export function daysUntil(date: Date | string | null): number | null {
	if (!date) {
		return null
	}
	const today = new Date()
	today.setHours(0, 0, 0, 0)
	const day = new Date(date)
	day.setHours(0, 0, 0, 0)
	return Math.round((day.getTime() - today.getTime()) / 864e5)
}
