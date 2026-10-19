import AbstractService from './abstractService'
import {apiV2Url} from '@/helpers/fetcher'
import TeamMemberModel from '@/models/teamMember'
import type {ITeamMember} from '@/modelTypes/ITeamMember'

export default class TeamMemberService extends AbstractService<ITeamMember> {
	constructor() {
		super({
			create: '/teams/{teamId}/members',
			delete: '/teams/{teamId}/members/{username}',
			update: '/teams/{teamId}/members/{username}/admin',
		})
	}

	modelFactory(data) {
		return new TeamMemberModel(data)
	}

	async setRole(teamId: number, username: string, role: string): Promise<string> {
		const cancel = this.setLoading()
		try {
			const {data} = await this.http.put(
				apiV2Url(`teams/${teamId}/members/${encodeURIComponent(username)}/role`),
				{role},
			)
			return data.role
		} finally {
			cancel()
		}
	}

	beforeCreate(model) {
		model.userId = model.id // The api wants to get the user id as user_Id
		model.admin = model.admin === null ? false : model.admin
		return model
	}
}
