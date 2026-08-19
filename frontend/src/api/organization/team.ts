import { get, post, put, del } from '@/utils/request'

export interface Department {
  id: string
  tenant_id: number
  name: string
  code: string
  status: string
}

export interface Team {
  id: string
  tenant_id: number
  department_id: string
  name: string
  code: string
  status: string
}

export interface TeamMember {
  id?: number
  team_id: string
  tenant_id: number
  user_id: string
  role?: string
  status?: string
}

export interface TeamAgent {
  id?: number
  team_id: string
  tenant_id: number
  agent_id: string
  status?: string
}

type ApiList<T> = { success: boolean; data?: T[]; message?: string }

export const listDepartments = () => get<ApiList<Department>>('/api/v1/organization/departments')
export const createDepartment = (data: Pick<Department, 'name' | 'code'>) =>
  post<{ success: boolean; data?: Department }>('/api/v1/organization/departments', data)

export const listTeams = (departmentId?: string) => {
  const query = departmentId ? `?department_id=${encodeURIComponent(departmentId)}` : ''
  return get<ApiList<Team>>(`/api/v1/organization/teams${query}`)
}
export const createTeam = (data: { department_id: string; name: string; code: string }) =>
  post<{ success: boolean; data?: Team }>('/api/v1/organization/teams', data)
export const updateTeam = (id: string, data: Pick<Team, 'department_id' | 'name' | 'code'>) =>
  put<{ success: boolean }>(`/api/v1/organization/teams/${encodeURIComponent(id)}`, data)
export const deleteTeam = (id: string) =>
  del<{ success: boolean }>(`/api/v1/organization/teams/${encodeURIComponent(id)}`)

export const listTeamMembers = (teamId: string) =>
  get<ApiList<TeamMember>>(`/api/v1/organization/teams/${encodeURIComponent(teamId)}/members`)
export const addTeamMember = (teamId: string, userId: string) =>
  post<{ success: boolean }>(`/api/v1/organization/teams/${encodeURIComponent(teamId)}/members`, { user_id: userId })
export const removeTeamMember = (teamId: string, userId: string) =>
  del<{ success: boolean }>(`/api/v1/organization/teams/${encodeURIComponent(teamId)}/members/${encodeURIComponent(userId)}`)

export const listTeamAgents = (teamId: string) =>
  get<ApiList<TeamAgent>>(`/api/v1/organization/teams/${encodeURIComponent(teamId)}/agents`)
export const addTeamAgent = (teamId: string, agentId: string) =>
  post<{ success: boolean }>(`/api/v1/organization/teams/${encodeURIComponent(teamId)}/agents`, { agent_id: agentId })
export const removeTeamAgent = (teamId: string, agentId: string) =>
  del<{ success: boolean }>(`/api/v1/organization/teams/${encodeURIComponent(teamId)}/agents/${encodeURIComponent(agentId)}`)
